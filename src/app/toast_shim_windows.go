//go:build windows

package main

// toast_shim_windows.go provides raw WinRT helpers that jackmordaunt/go-toast/v2
// does not expose:
//   - shimPushWithTagGroup: Push a toast notification with Tag + Group set on the
//     IToastNotification COM object (NOTIF-05 prerequisite — Tag enables
//     per-toast removal from Action Center).
//   - shimClearToast: Remove an individual toast from Action Center via
//     ToastNotificationHistory.Remove(tag, group, aumid).
//
// Both functions replicate the WinRT COM call patterns documented in the
// jackmordaunt library's ARCHITECTURE.md and its internal/winrt/* generated
// bindings — see those files for the VTable and IInspectable patterns.
//
// Privacy invariant (QUAL-03): NO raw MailMessage content crosses this layer.
// Callers pass only scalars: aumid, tag (emailId hash), group, and the toast
// XML built by toast_windows.go from privacy-sanitized fields only.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"syscall"
	"unsafe"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
	toasttmpl "git.sr.ht/~jackmordaunt/go-toast/v2/tmpl"
	ole "github.com/go-ole/go-ole"
)

// WinRT GUID strings for interfaces not exposed by jackmordaunt/go-toast.
//
// IToastNotification2 adds put_Tag/put_Group/put_SuppressPopup methods.
// Reference: learn.microsoft.com/en-us/uwp/api/windows.ui.notifications.itoastnotification2
const guidIToastNotification2 = "{9DFB9FD1-143A-490E-90BF-B9FBA7132DE7}"

// Well-known WinRT interface GUID for IToastNotificationManagerStatics2.
// Provides GetHistory() which returns IToastNotificationHistory.
// Reference: learn.microsoft.com/en-us/uwp/api/windows.ui.notifications.itoastnotificationmanagerstatics2
const guidIToastNotificationManagerStatics2 = "{7AB93C52-0E48-4750-BA9D-1A4113981847}"

// IToastNotification2 vtable layout mirrors Windows.UI.Notifications.IToastNotification2.
// Inherits from IInspectable (3 IUnknown methods + 3 IInspectable methods = 6 before ours).
// See Windows.UI.Notifications.h for exact layout.
type iToastNotification2 struct {
	ole.IInspectable
}

type iToastNotification2Vtbl struct {
	ole.IInspectableVtbl
	// IToastNotification2 methods (in header order):
	PutTag           uintptr
	GetTag           uintptr
	PutGroup         uintptr
	GetGroup         uintptr
	PutSuppressPopup uintptr
	GetSuppressPopup uintptr
}

func (v *iToastNotification2) VTable() *iToastNotification2Vtbl {
	return (*iToastNotification2Vtbl)(unsafe.Pointer(v.RawVTable))
}

// IToastNotificationManagerStatics2 vtable for GetHistory.
// learn.microsoft.com/en-us/uwp/api/windows.ui.notifications.itoastnotificationmanagerstatics2
type iToastNotificationManagerStatics2 struct {
	ole.IInspectable
}

type iToastNotificationManagerStatics2Vtbl struct {
	ole.IInspectableVtbl
	GetHistory uintptr
}

func (v *iToastNotificationManagerStatics2) VTable() *iToastNotificationManagerStatics2Vtbl {
	return (*iToastNotificationManagerStatics2Vtbl)(unsafe.Pointer(v.RawVTable))
}

// IToastNotificationHistory vtable for Remove.
// learn.microsoft.com/en-us/uwp/api/windows.ui.notifications.toastnotificationhistory
type iToastNotificationHistory struct {
	ole.IInspectable
}

const guidIToastNotificationHistory = "{2331101B-4F6F-4B99-98A5-DBBA7122F9B6}"

type iToastNotificationHistoryVtbl struct {
	ole.IInspectableVtbl
	RemoveGroup uintptr
	Remove      uintptr // Remove(tag, group, aumid)
}

func (v *iToastNotificationHistory) VTable() *iToastNotificationHistoryVtbl {
	return (*iToastNotificationHistoryVtbl)(unsafe.Pointer(v.RawVTable))
}

// shimPushWithTagGroup builds the toast XML from a jackmordaunt Notification,
// creates the WinRT IToastNotification object, sets Tag + Group, and shows it
// via ToastNotifier.Show. Falls back silently if Tag/Group cannot be set (Win7).
//
// Design: we re-use the library's own XML template (accessed via toasttmpl.XMLTemplate)
// rather than rolling our own XML builder. The COM push path is re-implemented
// here so we can intercept the IToastNotification before it is shown and inject
// put_Tag / put_Group.
func shimPushWithTagGroup(aumid string, n toast.Notification, tag, group string) error {
	// Named payload, not "xml": ARRICKS-30 imports encoding/xml into this
	// file, and a local named xml would shadow the package.
	payload, err := buildToastXML(n)
	if err != nil {
		return fmt.Errorf("toast shim: build xml: %w", err)
	}

	// Initialize WinRT (idempotent; S_FALSE = already initialized is acceptable).
	if err := ole.RoInitialize(1); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok && oleErr.Code() != 0x00000001 {
			return fmt.Errorf("toast shim: RoInitialize: %w", err)
		}
	}

	// Load the XML document.
	doc, err := newXmlDocument(payload)
	if err != nil {
		return fmt.Errorf("toast shim: load xml: %w", err)
	}
	defer doc.Release()

	// Create the ToastNotifier for our AUMID.
	notifier, err := createToastNotifierForAumid(aumid)
	if err != nil {
		return fmt.Errorf("toast shim: create notifier: %w", err)
	}
	defer notifier.Release()

	// Create the IToastNotification from our XML document.
	notification, err := createToastNotificationFromDoc(doc)
	if err != nil {
		return fmt.Errorf("toast shim: create notification: %w", err)
	}
	defer notification.Release()

	// Set Tag + Group via IToastNotification2 QueryInterface.
	if err := setTagGroup(notification, tag, group); err != nil {
		// Non-fatal: on Win7 or restricted environments, Tag/Group may not be
		// available. Log the error but still show the toast without tag/group.
		logError("toast shim: set tag/group (non-fatal): %v", err)
	}

	// Show the notification.
	if err := showToast(notifier, notification); err != nil {
		return fmt.Errorf("toast shim: show: %w", err)
	}
	return nil
}

// shimClearToast removes a specific toast from Action Center via
// ToastNotificationHistory.Remove(tag, group, aumid).
// NOTIF-05: called after MarkProcessed/Delete to prevent stale toasts.
func shimClearToast(aumid, tag, group string) error {
	if err := ole.RoInitialize(1); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok && oleErr.Code() != 0x00000001 {
			return fmt.Errorf("toast shim clear: RoInitialize: %w", err)
		}
	}

	// Get IToastNotificationManagerStatics2 factory.
	inspectable, err := ole.RoGetActivationFactory(
		"Windows.UI.Notifications.ToastNotificationManager",
		ole.NewGUID(guidIToastNotificationManagerStatics2),
	)
	if err != nil {
		return fmt.Errorf("toast shim clear: get statics2 factory: %w", err)
	}
	defer inspectable.Release()

	statics2 := (*iToastNotificationManagerStatics2)(unsafe.Pointer(inspectable))

	// Call GetHistory() to get the IToastNotificationHistory.
	var historyPtr *iToastNotificationHistory
	hr, _, _ := syscall.SyscallN(
		statics2.VTable().GetHistory,
		uintptr(unsafe.Pointer(statics2)),   // this
		uintptr(unsafe.Pointer(&historyPtr)), // out IToastNotificationHistory
	)
	if hr != 0 {
		return fmt.Errorf("toast shim clear: GetHistory HRESULT 0x%x", hr)
	}
	if historyPtr == nil {
		return fmt.Errorf("toast shim clear: GetHistory returned nil")
	}
	defer historyPtr.Release()

	// QueryInterface for IToastNotificationHistory (may be same ptr on win10+).
	histItf, err := historyPtr.QueryInterface(ole.NewGUID(guidIToastNotificationHistory))
	if err != nil {
		return fmt.Errorf("toast shim clear: QueryInterface IToastNotificationHistory: %w", err)
	}
	defer histItf.Release()
	hist := (*iToastNotificationHistory)(unsafe.Pointer(histItf))

	// Remove(tag, group, applicationId) — all three required for targeted removal.
	tagHStr, err := ole.NewHString(tag)
	if err != nil {
		return fmt.Errorf("toast shim clear: NewHString tag: %w", err)
	}
	defer ole.DeleteHString(tagHStr)

	groupHStr, err := ole.NewHString(group)
	if err != nil {
		return fmt.Errorf("toast shim clear: NewHString group: %w", err)
	}
	defer ole.DeleteHString(groupHStr)

	aumidHStr, err := ole.NewHString(aumid)
	if err != nil {
		return fmt.Errorf("toast shim clear: NewHString aumid: %w", err)
	}
	defer ole.DeleteHString(aumidHStr)

	hr, _, _ = syscall.SyscallN(
		hist.VTable().Remove,
		uintptr(unsafe.Pointer(hist)), // this
		uintptr(tagHStr),              // tag (HSTRING)
		uintptr(groupHStr),            // group (HSTRING)
		uintptr(aumidHStr),            // applicationId (HSTRING)
	)
	if hr != 0 {
		return fmt.Errorf("toast shim clear: Remove HRESULT 0x%x", hr)
	}
	return nil
}

// ---- Internal helpers ----

// buildToastXML generates the WinRT toast XML from a toast.Notification using
// the library's exported toasttmpl.XMLTemplate. Applies the same defaults as
// the library's internal applyDefaults() method.
//
// ARRICKS-30 — validate, then escape only if needed.
//
// Arrival toasts had NEVER worked. Every "toast: arrival push failed ...
// LoadXml HRESULT 0xc00ce50d" in app.log, going back to the earliest logs we
// have, is a push that died before it ever reached Action Center — meaning
// the queue's primary manual-mode surface, including its "Create draft"
// button, was silently dead on every machine.
//
// What isolates it: the arrival toast is the only one whose XML carries a raw
// ampersand — ActivationArguments and both action Arguments are built as
// "action=open&emailId=..." (toast_windows.go). The success, error and summary
// toasts use bare "action=open", and those get all the way to put_Tag. A bare
// & inside an XML attribute is a syntax error. CI proved the exact template
// behavior (first run of toast_xml_windows_test.go): TEXT NODES (Title, Body)
// come back escaped by the template itself, ATTRIBUTE VALUES (launch,
// arguments) come back raw — which is exactly why only the arrival toast,
// the sole toast with an & in an attribute, ever failed. Intermittency in
// the log is a red herring: emitArrivalToast returns early when the window
// is visible or the app is paused, so the quiet stretches are toasts never
// attempted at all.
//
// Rather than blind-escape (which would double-escape if the upstream
// template ever starts escaping, turning every "&" in a subject into
// "&amp;" on screen), render first and escape only when the result does not
// parse. Correct under both behaviors, and a no-op for the toasts that
// already work.
func buildToastXML(n toast.Notification) (string, error) {
	if n.ActivationType == "" {
		n.ActivationType = toast.Foreground
	}
	if n.Duration == "" {
		n.Duration = toast.Short // "short"
	}
	if n.Audio == "" {
		n.Audio = toast.Default
	}
	raw, err := renderToastXML(n)
	if err != nil {
		return "", err
	}
	if wellFormedXML(raw) {
		return raw, nil
	}
	escaped, err := renderToastXML(escapeToastFields(n))
	if err != nil {
		return "", err
	}
	if !wellFormedXML(escaped) {
		// Something other than our text fields is malformed — the template
		// itself, or a field we do not populate. Fail loudly rather than
		// handing XmlLite a payload we know it will reject.
		return "", fmt.Errorf("toast xml: still malformed after escaping")
	}
	return escaped, nil
}

// renderToastXML executes the upstream template. Split out so buildToastXML
// can run it twice (raw, then escaped) without duplicating the error wrap.
func renderToastXML(n toast.Notification) (string, error) {
	var buf bytes.Buffer
	if err := toasttmpl.XMLTemplate.Execute(&buf, n); err != nil {
		return "", fmt.Errorf("toast xml template: %w", err)
	}
	return buf.String(), nil
}

// wellFormedXML reports whether s parses as XML end to end.
//
// Go's parser is not XmlLite, so this is a gate rather than an oracle, and
// both ways it can be wrong are safe: a false "well-formed" leaves today's
// behavior exactly as it is, and a false "malformed" only escapes text that
// decodes back to the identical string. Deliberately returns a bool and not
// the error — the parse error can quote the offending run of the document,
// which for a toast means the email subject, and house rule 7 says subjects
// never reach a log.
func wellFormedXML(s string) bool {
	dec := xml.NewDecoder(strings.NewReader(s))
	dec.Strict = true
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
	}
}

// escapeToastFields returns a copy of n with the ATTRIBUTE-bound fields
// XML-escaped — and only those. The upstream template escapes text nodes
// (Title, Body) on its own but leaves attribute values raw, so escaping
// Title/Body here double-escapes them into literal "&amp;" on screen (CI
// caught exactly that on this file's first run). Escaping is
// value-preserving for the fields we do touch: the XML parser decodes
// "&amp;" back to "&" before the activation callback ever sees the
// arguments, so action routing is unaffected.
//
// n arrives by value, but its Actions slice still shares a backing array with
// the caller's — hence the explicit copy. Mutating it in place would corrupt
// the caller's notification on the retry path.
func escapeToastFields(n toast.Notification) toast.Notification {
	n.Icon = escapeXMLValue(n.Icon)
	n.ActivationArguments = escapeXMLValue(n.ActivationArguments)
	if len(n.Actions) > 0 {
		actions := make([]toast.Action, len(n.Actions))
		copy(actions, n.Actions)
		for i := range actions {
			actions[i].Content = escapeXMLValue(actions[i].Content)
			actions[i].Arguments = escapeXMLValue(actions[i].Arguments)
		}
		n.Actions = actions
	}
	return n
}

// escapeXMLValue escapes one string for use as XML text or an attribute
// value. xml.EscapeText also folds control characters and invalid code
// points, which matters because subjects arrive from scanner firmware.
func escapeXMLValue(s string) string {
	if s == "" {
		return ""
	}
	var buf bytes.Buffer
	// Only error path is the writer's; bytes.Buffer never fails.
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// IXmlDocumentIO is the Windows.Data.Xml.Dom.IXmlDocumentIO interface.
// GUID: {6CD0E74E-EE65-4489-9EBF-CA43E87BA637}
// Provides LoadXml for loading XML text into an XmlDocument.
type iXmlDocumentIO struct {
	ole.IInspectable
}

const guidIXmlDocumentIO = "{6CD0E74E-EE65-4489-9EBF-CA43E87BA637}"

type iXmlDocumentIOVtbl struct {
	ole.IInspectableVtbl
	LoadXml             uintptr
	LoadXmlWithSettings uintptr
	SaveToFileAsync     uintptr
}

func (v *iXmlDocumentIO) VTable() *iXmlDocumentIOVtbl {
	return (*iXmlDocumentIOVtbl)(unsafe.Pointer(v.RawVTable))
}

// newXmlDocument activates Windows.Data.Xml.Dom.XmlDocument and loads the
// provided XML string into it via IXmlDocumentIO.LoadXml.
func newXmlDocument(xmlStr string) (*ole.IUnknown, error) {
	inspectable, err := ole.RoActivateInstance("Windows.Data.Xml.Dom.XmlDocument")
	if err != nil {
		return nil, fmt.Errorf("activate XmlDocument: %w", err)
	}

	ioItf, err := inspectable.QueryInterface(ole.NewGUID(guidIXmlDocumentIO))
	if err != nil {
		inspectable.Release()
		return nil, fmt.Errorf("QueryInterface IXmlDocumentIO: %w", err)
	}
	defer ioItf.Release()
	io := (*iXmlDocumentIO)(unsafe.Pointer(ioItf))

	xmlHStr, err := ole.NewHString(xmlStr)
	if err != nil {
		inspectable.Release()
		return nil, fmt.Errorf("NewHString xml: %w", err)
	}
	defer ole.DeleteHString(xmlHStr)

	hr, _, _ := syscall.SyscallN(
		io.VTable().LoadXml,
		uintptr(unsafe.Pointer(io)),
		uintptr(xmlHStr),
	)
	if hr != 0 {
		inspectable.Release()
		return nil, fmt.Errorf("LoadXml HRESULT 0x%x", hr)
	}

	return (*ole.IUnknown)(unsafe.Pointer(inspectable)), nil
}

// guidIToastNotificationFactory is the IID for IToastNotificationFactory.
// Provides CreateToastNotification.
const guidIToastNotificationFactory = "{04124B20-82C6-4229-B109-FD9ED4662B53}"

// toastNotification wraps the raw COM pointer to an IToastNotification object.
type toastNotification struct {
	ptr *ole.IUnknown
}

func (t *toastNotification) Release() { t.ptr.Release() }

// createToastNotificationFromDoc calls the WinRT factory to create an
// IToastNotification from an XmlDocument IUnknown pointer.
func createToastNotificationFromDoc(doc *ole.IUnknown) (*toastNotification, error) {
	factory, err := ole.RoGetActivationFactory(
		"Windows.UI.Notifications.ToastNotification",
		ole.NewGUID(guidIToastNotificationFactory),
	)
	if err != nil {
		return nil, fmt.Errorf("get notification factory: %w", err)
	}
	defer factory.Release()

	// The factory vtable: IInspectable (6 entries) + CreateToastNotification.
	type notifFactoryVtbl struct {
		ole.IInspectableVtbl
		CreateToastNotification uintptr
	}
	type notifFactory struct {
		ole.IInspectable
	}
	f := (*notifFactory)(unsafe.Pointer(factory))
	vtbl := (*notifFactoryVtbl)(unsafe.Pointer(f.RawVTable))

	var outPtr *ole.IUnknown
	hr, _, _ := syscall.SyscallN(
		vtbl.CreateToastNotification,
		uintptr(unsafe.Pointer(f)),       // this: IToastNotificationFactory instance pointer
		uintptr(unsafe.Pointer(doc)),     // in: IXmlDocument
		uintptr(unsafe.Pointer(&outPtr)), // out: IToastNotification
	)
	if hr != 0 {
		return nil, fmt.Errorf("CreateToastNotification HRESULT 0x%x", hr)
	}
	return &toastNotification{ptr: outPtr}, nil
}

// setTagGroup calls QueryInterface for IToastNotification2 and invokes
// put_Tag + put_Group on the notification object. Tag enables per-notification
// removal from Action Center; Group enables go-mapi notification collapse.
func setTagGroup(n *toastNotification, tag, group string) error {
	itf, err := n.ptr.QueryInterface(ole.NewGUID(guidIToastNotification2))
	if err != nil {
		return fmt.Errorf("QueryInterface IToastNotification2: %w", err)
	}
	defer itf.Release()
	n2 := (*iToastNotification2)(unsafe.Pointer(itf))

	tagHStr, err := ole.NewHString(tag)
	if err != nil {
		return fmt.Errorf("NewHString tag: %w", err)
	}
	defer ole.DeleteHString(tagHStr)

	hr, _, _ := syscall.SyscallN(
		n2.VTable().PutTag,
		uintptr(unsafe.Pointer(n2)),
		uintptr(tagHStr),
	)
	if hr != 0 {
		return fmt.Errorf("put_Tag HRESULT 0x%x", hr)
	}

	groupHStr, err := ole.NewHString(group)
	if err != nil {
		return fmt.Errorf("NewHString group: %w", err)
	}
	defer ole.DeleteHString(groupHStr)

	hr, _, _ = syscall.SyscallN(
		n2.VTable().PutGroup,
		uintptr(unsafe.Pointer(n2)),
		uintptr(groupHStr),
	)
	if hr != 0 {
		return fmt.Errorf("put_Group HRESULT 0x%x", hr)
	}
	return nil
}

// guidIToastNotificationManagerStatics5 provides GetDefault for the
// ToastNotificationManagerForUser path (Windows 10 1607+).
const guidIToastNotificationManagerStatics5 = "{D6F5F569-D40D-407C-8989-88CAB42CFD14}"

type iToastManagerStatics5 struct {
	ole.IInspectable
}

type iToastManagerStatics5Vtbl struct {
	ole.IInspectableVtbl
	GetDefault uintptr
}

func (v *iToastManagerStatics5) VTable() *iToastManagerStatics5Vtbl {
	return (*iToastManagerStatics5Vtbl)(unsafe.Pointer(v.RawVTable))
}

// guidIToastNotificationManagerForUser is the AUMID-aware notifier factory.
const guidIToastNotificationManagerForUser = "{79AB57F6-43FE-487B-8A7F-99567200AE94}"

type iToastManagerForUserVtbl struct {
	ole.IInspectableVtbl
	CreateToastNotifier       uintptr
	CreateToastNotifierWithId uintptr
	GetHistory                uintptr
	GetUser                   uintptr
}

type iToastManagerForUser struct {
	ole.IInspectable
}

func (v *iToastManagerForUser) VTable() *iToastManagerForUserVtbl {
	return (*iToastManagerForUserVtbl)(unsafe.Pointer(v.RawVTable))
}

// createToastNotifierForAumid returns an IToastNotifier for the given AUMID
// using the static GetDefault + CreateToastNotifierWithId path.
func createToastNotifierForAumid(aumid string) (*ole.IUnknown, error) {
	factory, err := ole.RoGetActivationFactory(
		"Windows.UI.Notifications.ToastNotificationManager",
		ole.NewGUID(guidIToastNotificationManagerStatics5),
	)
	if err != nil {
		return nil, fmt.Errorf("get manager statics5: %w", err)
	}
	defer factory.Release()

	s5 := (*iToastManagerStatics5)(unsafe.Pointer(factory))
	var managerPtr *ole.IUnknown
	hr, _, _ := syscall.SyscallN(
		s5.VTable().GetDefault,
		0,                                    // static: no this
		uintptr(unsafe.Pointer(&managerPtr)), // out: ToastNotificationManagerForUser
	)
	if hr != 0 {
		return nil, fmt.Errorf("GetDefault HRESULT 0x%x", hr)
	}
	defer managerPtr.Release()

	// QI for IToastNotificationManagerForUser to call CreateToastNotifierWithId.
	forUserItf, err := managerPtr.QueryInterface(ole.NewGUID(guidIToastNotificationManagerForUser))
	if err != nil {
		return nil, fmt.Errorf("QueryInterface IToastNotificationManagerForUser: %w", err)
	}
	defer forUserItf.Release()
	forUser := (*iToastManagerForUser)(unsafe.Pointer(forUserItf))

	aumidHStr, err := ole.NewHString(aumid)
	if err != nil {
		return nil, fmt.Errorf("NewHString aumid: %w", err)
	}
	defer ole.DeleteHString(aumidHStr)

	var notifierPtr *ole.IUnknown
	hr, _, _ = syscall.SyscallN(
		forUser.VTable().CreateToastNotifierWithId,
		uintptr(unsafe.Pointer(forUser)),      // this
		uintptr(aumidHStr),                    // in: applicationId (HSTRING)
		uintptr(unsafe.Pointer(&notifierPtr)), // out: IToastNotifier
	)
	if hr != 0 {
		return nil, fmt.Errorf("CreateToastNotifierWithId HRESULT 0x%x", hr)
	}
	return notifierPtr, nil
}

// guidIToastNotifier is the IID for IToastNotifier.
// Provides Show / Hide / GetSetting.
const guidIToastNotifier = "{75927B93-03F3-41EC-91D3-6E5BAC1B38E7}"

type iToastNotifierVtbl struct {
	ole.IInspectableVtbl
	Show                           uintptr
	Hide                           uintptr
	GetSetting                     uintptr
	AddToSchedule                  uintptr
	RemoveFromSchedule             uintptr
	GetScheduledToastNotifications uintptr
}

type iToastNotifier struct {
	ole.IInspectable
}

func (v *iToastNotifier) VTable() *iToastNotifierVtbl {
	return (*iToastNotifierVtbl)(unsafe.Pointer(v.RawVTable))
}

// showToast invokes IToastNotifier.Show on the given notification.
func showToast(notifier *ole.IUnknown, n *toastNotification) error {
	itf, err := notifier.QueryInterface(ole.NewGUID(guidIToastNotifier))
	if err != nil {
		return fmt.Errorf("QueryInterface IToastNotifier: %w", err)
	}
	defer itf.Release()
	v := (*iToastNotifier)(unsafe.Pointer(itf))

	hr, _, _ := syscall.SyscallN(
		v.VTable().Show,
		uintptr(unsafe.Pointer(v)),     // this
		uintptr(unsafe.Pointer(n.ptr)), // in: IToastNotification
	)
	if hr != 0 {
		return fmt.Errorf("Show HRESULT 0x%x", hr)
	}
	return nil
}
