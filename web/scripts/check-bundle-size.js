#!/usr/bin/env node

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { readFileSync, readdirSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'
import { gzipSync } from 'zlib'

const __dirname = dirname(fileURLToPath(import.meta.url))
const assetsDir = join(__dirname, '..', 'dist', 'assets')

let files
try {
  files = readdirSync(assetsDir)
} catch {
  console.error('dist/assets not found — run npm run build first')
  process.exit(1)
}

const initial = files.filter((f) => f.endsWith('.js') || f.endsWith('.css'))

let totalGzip = 0
for (const file of initial) {
  totalGzip += gzipSync(readFileSync(join(assetsDir, file))).length
}

const kb = (totalGzip / 1024).toFixed(1)
console.log(`Bundle size: ${kb} KB gzipped`)

if (totalGzip > 512000) {
  console.error(`Bundle size ${kb} KB exceeds limit of 500 KB`)
  process.exit(1)
}
