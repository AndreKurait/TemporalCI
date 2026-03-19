#!/usr/bin/env node
/**
 * site/scripts/validate.mjs — Post-build checks for the TemporalCI site.
 * Run after `npm run build` to catch common bugs before deploy.
 *
 * Checks:
 *  1. No hardcoded dark-mode hex colors in custom CSS (use --sl-color-* vars)
 *  2. No double-wrapped <p> tags from MDX rendering
 *  3. No broken internal links (href to /TemporalCI/... that don't resolve)
 *  4. All pages have OG meta tags
 *  5. No ASCII box-drawing characters outside <pre> (fragile rendering)
 *  6. Landing page has all expected sections
 */

import { readFileSync, readdirSync, existsSync } from 'fs';
import { join } from 'path';

const DIST = new URL('../dist/', import.meta.url).pathname;
let errors = 0;
let warnings = 0;

function fail(msg) { console.error(`  ❌ ${msg}`); errors++; }
function warn(msg) { console.warn(`  ⚠️  ${msg}`); warnings++; }
function pass(msg) { console.log(`  ✅ ${msg}`); }

// Collect all HTML files
function findHtml(dir) {
  let files = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) files.push(...findHtml(full));
    else if (entry.name.endsWith('.html')) files.push(full);
  }
  return files;
}

const htmlFiles = findHtml(DIST);
const allPaths = new Set();
for (const f of htmlFiles) {
  // /dist/getting-started/quickstart/index.html → /getting-started/quickstart/
  const rel = f.replace(DIST, '/').replace(/index\.html$/, '').replace(/\.html$/, '/');
  allPaths.add(rel);
}

console.log(`\nValidating ${htmlFiles.length} pages...\n`);

// ── Check 1: No hardcoded dark hex in CSS ──
console.log('1. CSS hardcoded colors');
const cssFiles = readdirSync(join(DIST, '_astro')).filter(f => f.endsWith('.css'));
const BANNED_HEX = ['#0a0a0a', '#141414', '#1a1a1a', '#262626', '#fafafa', '#0d0d0d'];
for (const cssFile of cssFiles) {
  const css = readFileSync(join(DIST, '_astro', cssFile), 'utf8');
  // Only check our custom CSS portion (look for landing-page class)
  if (!css.includes('landing-page')) continue;
  // Extract just the landing-page related portion
  for (const hex of BANNED_HEX) {
    // Count occurrences outside of Starlight's own layer blocks
    const re = new RegExp(`(?<!--sl-color-[a-z-]*:)${hex.replace('#', '#')}`, 'gi');
    const matches = css.match(re);
    if (matches && matches.length > 0) {
      fail(`${cssFile}: found hardcoded color ${hex} (${matches.length}x) — use var(--sl-color-*) instead`);
    }
  }
}
if (errors === 0) pass('No hardcoded dark-mode colors in CSS');

// ── Check 2: No double-wrapped <p> tags ──
console.log('2. Double-wrapped <p> tags');
let doubleP = 0;
for (const f of htmlFiles) {
  const html = readFileSync(f, 'utf8');
  // Strip SVG elements first to avoid <path matching as <p
  const noSvg = html.replace(/<svg[\s\S]*?<\/svg>/g, '');
  const matches = noSvg.match(/<p[^>]*>\s*<p/g);
  if (matches) {
    const rel = f.replace(DIST, '');
    fail(`${rel}: ${matches.length} double-wrapped <p> tag(s)`);
    doubleP += matches.length;
  }
}
if (doubleP === 0) pass('No double-wrapped <p> tags');

// ── Check 3: Internal link validation ──
console.log('3. Internal links');
let brokenLinks = 0;
for (const f of htmlFiles) {
  const html = readFileSync(f, 'utf8');
  const rel = f.replace(DIST, '');
  // Find all internal links
  const linkRe = /href="(\/TemporalCI\/[^"#?]*)"/g;
  let match;
  while ((match = linkRe.exec(html)) !== null) {
    const target = match[1].replace('/TemporalCI', '');
    const normalized = target.endsWith('/') ? target : target + '/';
    if (!allPaths.has(normalized) && !existsSync(join(DIST, target))) {
      fail(`${rel}: broken link → ${match[1]}`);
      brokenLinks++;
    }
  }
}
if (brokenLinks === 0) pass('All internal links resolve');

// ── Check 4: OG meta tags ──
console.log('4. OG meta tags');
let missingOg = 0;
for (const f of htmlFiles) {
  if (f.includes('404')) continue;
  const html = readFileSync(f, 'utf8');
  const rel = f.replace(DIST, '');
  if (!html.includes('og:image')) {
    fail(`${rel}: missing og:image meta tag`);
    missingOg++;
  }
}
if (missingOg === 0) pass('All pages have og:image');

// ── Check 5: No ASCII box-drawing outside <pre>/<code> ──
console.log('5. ASCII box-drawing characters');
const BOX_CHARS = /[┌┐└┘├┤┬┴┼─│]/;
for (const f of htmlFiles) {
  const html = readFileSync(f, 'utf8');
  const rel = f.replace(DIST, '');
  // Strip <pre>, <code>, and <svg> blocks, then check for box chars
  const stripped = html
    .replace(/<pre[\s\S]*?<\/pre>/g, '')
    .replace(/<code[\s\S]*?<\/code>/g, '')
    .replace(/<svg[\s\S]*?<\/svg>/g, '');
  if (BOX_CHARS.test(stripped)) {
    warn(`${rel}: ASCII box-drawing chars outside <pre>/<code> — use Mermaid instead`);
  }
}

// ── Check 6: Landing page sections ──
console.log('6. Landing page sections');
const indexHtml = readFileSync(join(DIST, 'index.html'), 'utf8');
const requiredSections = [
  ['hero-section', 'Hero'],
  ['pain-grid', 'Problem/Pain'],
  ['config-showcase', 'Config Showcase'],
  ['flow-steps', 'How It Works flow'],
  ['feature-grid', 'Feature Grid'],
  ['comparison-table', 'Comparison Table'],
  ['quickstart-steps', 'Quickstart'],
  ['footer-cta', 'Footer CTA'],
];
for (const [cls, name] of requiredSections) {
  if (!indexHtml.includes(cls)) {
    fail(`Landing page missing section: ${name} (class="${cls}")`);
  }
}
if (errors === 0 || !requiredSections.some(([cls]) => !indexHtml.includes(cls))) {
  pass('All landing page sections present');
}

// ── Summary ──
console.log(`\n${'─'.repeat(50)}`);
if (errors > 0) {
  console.error(`\n💥 ${errors} error(s), ${warnings} warning(s)\n`);
  process.exit(1);
} else {
  console.log(`\n✅ All checks passed (${warnings} warning(s))\n`);
}
