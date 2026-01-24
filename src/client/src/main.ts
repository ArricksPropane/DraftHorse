/**
 * Main Electron process
 * Handles window management, system tray, and IPC
 */

import { app, BrowserWindow, Menu, ipcMain, Tray, nativeImage } from "electron";
import * as path from "path";
import * as os from "os";
import Store from "electron-store";
import { MailQueue, MailMessage } from "./mail-queue";
import { FileWatcher } from "./watcher";
import { GmailSender } from "./gmail-sender";

// Global references
let mainWindow: BrowserWindow | null = null;
let tray: Tray | null = null;
const queue = new MailQueue();
let watcher: FileWatcher;
const store = new Store();

// Check if this is the first instance
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
}

/**
 * Create the main window
 */
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 900,
    height: 700,
    minWidth: 600,
    minHeight: 400,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
    icon: path.join(__dirname, "../assets/icon.png"),
  });

  // Load the renderer
  const startUrl = `file://${path.join(__dirname, "../renderer/index.html")}`;
  mainWindow.loadURL(startUrl);

  // Open DevTools in development
  if (process.env.NODE_ENV === "development") {
    mainWindow.webContents.openDevTools();
  }

  // Handle window closed
  mainWindow.on("closed", () => {
    mainWindow = null;
  });

  // Subscribe to queue changes and send updates to renderer
  queue.subscribe((updatedQueue) => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.send("mail:queueUpdated", updatedQueue);
    }
    updateTrayMenu();
  });
}

/**
 * Create the system tray icon and menu
 */
function createTray() {
  // Create a simple tray icon (can be replaced with actual PNG later)
  const icon = nativeImage.createEmpty();
  tray = new Tray(icon);

  updateTrayMenu();

  tray.on("double-click", () => {
    showWindow();
  });
}

/**
 * Update the tray menu
 */
function updateTrayMenu() {
  if (!tray) {
    return;
  }

  const queueSize = queue.size();
  const contextMenu = Menu.buildFromTemplate([
    {
      label: `Pending Emails (${queueSize})`,
      enabled: false,
    },
    { type: "separator" },
    {
      label: "Show",
      click: () => showWindow(),
    },
    {
      label: "Settings",
      click: () => {
        if (mainWindow) {
          mainWindow.webContents.send("app:showSettings");
        }
      },
    },
    { type: "separator" },
    {
      label: "Quit",
      click: () => {
        app.quit();
      },
    },
  ]);

  tray.setContextMenu(contextMenu);
}

/**
 * Show the main window
 */
function showWindow() {
  if (mainWindow) {
    mainWindow.show();
    mainWindow.focus();
  }
}

/**
 * Initialize IPC handlers
 */
function initializeIpc() {
  // Get current queue
  ipcMain.handle("mail:getQueue", () => {
    return queue.getAll();
  });

  // Send an email
  ipcMain.handle("mail:send", async (event, { id, gmailToken }: { id: string; gmailToken: string }) => {
    const message = queue.getById(id);
    if (!message) {
      return false;
    }

    try {
      const sender = new GmailSender(gmailToken);
      await sender.sendMessage(message);
      queue.remove(id);
      return true;
    } catch (error) {
      console.error("Failed to send email:", error);
      return false;
    }
  });

  // Delete an email
  ipcMain.handle("mail:delete", (event, { id }: { id: string }) => {
    return queue.remove(id);
  });

  // Open settings
  ipcMain.handle("app:openSettings", () => {
    if (mainWindow) {
      mainWindow.webContents.send("app:showSettings");
    }
  });
}

/**
 * App event handlers
 */
app.on("ready", async () => {
  // Initialize file watcher
  watcher = new FileWatcher(queue);
  await watcher.start();

  // Create windows and tray
  createWindow();
  createTray();

  // Initialize IPC
  initializeIpc();
});

app.on("window-all-closed", () => {
  // On macOS, applications stay active until the user quits explicitly
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("activate", () => {
  // On macOS, re-create a window when the dock icon is clicked
  if (mainWindow === null) {
    createWindow();
  }
});

// Handle single instance (second instance tries to launch)
app.on("second-instance", () => {
  if (mainWindow) {
    if (mainWindow.isMinimized()) mainWindow.restore();
    mainWindow.focus();
  }
});

// Cleanup on exit
app.on("quit", async () => {
  if (watcher) {
    await watcher.stop();
  }
});
