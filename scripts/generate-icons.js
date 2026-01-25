#!/usr/bin/env node

import sharp from 'sharp';
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const iconsDir = join(__dirname, '../src/extension/public/icons');

// Ensure icons directory exists
if (!existsSync(iconsDir)) {
  mkdirSync(iconsDir, { recursive: true });
}

// Read SVG
const svgPath = join(iconsDir, 'icon.svg');
let svgContent;

if (existsSync(svgPath)) {
  svgContent = readFileSync(svgPath);
} else {
  // Create a simple colored square SVG as fallback
  svgContent = Buffer.from(`
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128">
      <rect width="128" height="128" fill="#4285f4" rx="16"/>
      <text x="64" y="80" font-family="Arial" font-size="64" fill="white" text-anchor="middle">M</text>
    </svg>
  `);
}

const sizes = [16, 32, 48, 128];

console.log('Generating PNG icons...');

for (const size of sizes) {
  const outputPath = join(iconsDir, `icon${size}.png`);
  await sharp(svgContent)
    .resize(size, size)
    .png()
    .toFile(outputPath);
  console.log(`  Created: icon${size}.png`);
}

console.log('Done!');
