/**
 * Unit tests for MailQueue
 */

import { MailQueue, MailMessage } from "../mail-queue";

describe("MailQueue", () => {
  let queue: MailQueue;

  beforeEach(() => {
    queue = new MailQueue();
  });

  describe("add", () => {
    it("should add a message to the queue", () => {
      const message: MailMessage = {
        id: "msg_123",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test",
        body: "Test body",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(message);
      expect(queue.size()).toBe(1);
      expect(queue.getById("msg_123")).toEqual(message);
    });

    it("should notify listeners when a message is added", () => {
      const listener = jest.fn();
      queue.subscribe(listener);

      const message: MailMessage = {
        id: "msg_123",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test",
        body: "Test body",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(message);
      expect(listener).toHaveBeenCalledWith([message]);
    });
  });

  describe("remove", () => {
    it("should remove a message from the queue", () => {
      const message: MailMessage = {
        id: "msg_123",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test",
        body: "Test body",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(message);
      expect(queue.size()).toBe(1);

      const removed = queue.remove("msg_123");
      expect(removed).toBe(true);
      expect(queue.size()).toBe(0);
    });

    it("should return false when removing non-existent message", () => {
      const removed = queue.remove("non_existent");
      expect(removed).toBe(false);
    });

    it("should notify listeners when a message is removed", () => {
      const listener = jest.fn();
      queue.subscribe(listener);

      const message: MailMessage = {
        id: "msg_123",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test",
        body: "Test body",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(message);
      queue.remove("msg_123");

      expect(listener).toHaveBeenLastCalledWith([]);
    });
  });

  describe("getAll", () => {
    it("should return all messages in the queue", () => {
      const msg1: MailMessage = {
        id: "msg_1",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test 1",
        body: "Body 1",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      const msg2: MailMessage = {
        id: "msg_2",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test 2",
        body: "Body 2",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(msg1);
      queue.add(msg2);

      const all = queue.getAll();
      expect(all).toHaveLength(2);
      expect(all).toContain(msg1);
      expect(all).toContain(msg2);
    });
  });

  describe("clear", () => {
    it("should clear all messages from the queue", () => {
      const message: MailMessage = {
        id: "msg_123",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test",
        body: "Test body",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(message);
      queue.clear();

      expect(queue.size()).toBe(0);
      expect(queue.getAll()).toEqual([]);
    });
  });

  describe("subscribe", () => {
    it("should return an unsubscribe function", () => {
      const listener = jest.fn();
      const unsubscribe = queue.subscribe(listener);

      const message: MailMessage = {
        id: "msg_123",
        version: 1,
        timestamp: new Date().toISOString(),
        subject: "Test",
        body: "Test body",
        bodyFormat: "plain",
        recipients: { to: [], cc: [], bcc: [] },
        attachments: [],
        originApp: "test.exe",
      };

      queue.add(message);
      expect(listener).toHaveBeenCalled();

      listener.mockClear();

      unsubscribe();
      queue.add(message);

      expect(listener).not.toHaveBeenCalled();
    });
  });
});
