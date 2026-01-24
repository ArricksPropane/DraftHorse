/**
 * File system watcher for intercepted MAPI messages
 * Monitors %TEMP%\go-mapi\ for new JSON files
 */

import * as fs from "fs";
import * as path from "path";
import * as os from "os";
import chokidar, { FSWatcher } from "chokidar";
import { JsonParser } from "./json-parser";
import { MailMessage, MailQueue } from "./mail-queue";

export interface WatcherConfig {
  watchDir?: string;
  processedDir?: string;
}

export class FileWatcher {
  private watcher: FSWatcher | null = null;
  private watchDir: string;
  private processedDir: string;
  private queue: MailQueue;
  private fileProcessingInProgress: Set<string> = new Set();

  constructor(queue: MailQueue, config?: WatcherConfig) {
    this.queue = queue;
    this.watchDir = config?.watchDir || this.getDefaultWatchDir();
    this.processedDir = config?.processedDir || path.join(this.watchDir, "processed");
  }

  /**
   * Start watching the directory
   */
  async start(): Promise<void> {
    // Ensure directories exist
    await this.ensureDirectories();

    // Initialize the watcher
    this.watcher = chokidar.watch(this.watchDir, {
      ignored: [
        /(^|[\/\\])\./,  // Ignore dotfiles
        /processed/,     // Ignore processed subdirectory
      ],
      persistent: true,
      awaitWriteFinish: {
        stabilityThreshold: 500,
        pollInterval: 100,
      },
    });

    // Set up event handlers
    this.watcher.on("add", (filePath) => {
      this.onFileAdded(filePath);
    });

    this.watcher.on("error", (error) => {
      console.error("Watcher error:", error);
    });

    console.log(`File watcher started for: ${this.watchDir}`);
  }

  /**
   * Stop watching the directory
   */
  async stop(): Promise<void> {
    if (this.watcher) {
      await this.watcher.close();
      this.watcher = null;
    }
    console.log("File watcher stopped");
  }

  /**
   * Get the watch directory
   */
  getWatchDir(): string {
    return this.watchDir;
  }

  /**
   * Handle a new file being added
   */
  private async onFileAdded(filePath: string): Promise<void> {
    // Check if this is a JSON file
    if (!filePath.endsWith(".json")) {
      return;
    }

    // Prevent duplicate processing
    if (this.fileProcessingInProgress.has(filePath)) {
      return;
    }

    this.fileProcessingInProgress.add(filePath);

    try {
      // Read the file
      let content: string;
      try {
        content = await fs.promises.readFile(filePath, "utf-8");
      } catch (error) {
        console.error(`Failed to read file ${filePath}:`, error);
        this.fileProcessingInProgress.delete(filePath);
        return;
      }

      // Parse and validate
      let message: MailMessage;
      try {
        message = JsonParser.parseAndValidate(content);
      } catch (error) {
        console.error(`Failed to parse message from ${filePath}:`, error);
        this.fileProcessingInProgress.delete(filePath);
        return;
      }

      // Add to queue
      this.queue.add(message);
      console.log(`Message added to queue: ${message.subject}`);

      // Move file to processed directory
      await this.moveToProcessed(filePath);
    } finally {
      this.fileProcessingInProgress.delete(filePath);
    }
  }

  /**
   * Move a processed file to the processed subdirectory
   */
  private async moveToProcessed(filePath: string): Promise<void> {
    try {
      const fileName = path.basename(filePath);
      const newPath = path.join(this.processedDir, fileName);
      await fs.promises.rename(filePath, newPath);
      console.log(`File moved to processed: ${newPath}`);
    } catch (error) {
      console.error(`Failed to move file to processed: ${error}`);
      // Don't re-throw; the file has been processed even if archival failed
    }
  }

  /**
   * Ensure required directories exist
   */
  private async ensureDirectories(): Promise<void> {
    try {
      // Create watch directory if it doesn't exist
      await fs.promises.mkdir(this.watchDir, { recursive: true });

      // Create processed subdirectory
      await fs.promises.mkdir(this.processedDir, { recursive: true });
    } catch (error) {
      console.error("Failed to create directories:", error);
      throw error;
    }
  }

  /**
   * Get the default watch directory (%TEMP%\go-mapi\)
   */
  private getDefaultWatchDir(): string {
    const tempDir = os.tmpdir();
    return path.join(tempDir, "go-mapi");
  }
}
