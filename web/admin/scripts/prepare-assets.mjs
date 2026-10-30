import { createHash } from 'node:crypto';
import { mkdir, readdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import sharp from 'sharp';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const publicDir = path.join(root, 'public');
const generatedDir = path.join(root, 'lib/generated');
const outputDir = path.join(publicDir, '_assets');
await mkdir(outputDir, { recursive: true });
await mkdir(generatedDir, { recursive: true });

async function walk(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = await Promise.all(entries.map(entry => entry.isDirectory()
    ? walk(path.join(dir, entry.name)) : [path.join(dir, entry.name)]));
  return files.flat().sort();
}

const assets = {};
let originalBytes = 0;
let optimizedBytes = 0;
for (const file of await walk(publicDir)) {
  const source = '/' + path.relative(publicDir, file).split(path.sep).join('/');
  if (!/^\/(?:mascots\/|assets\/(?:auth|demo)\/|login-bg|logo\.png$)/.test(source) || !/\.png$/.test(source)) continue;
  const input = await readFile(file);
  const logo = source === '/logo.png';
  const mascot = source.startsWith('/mascots/');
  // Keep pixel art exact; preserve transparent edges on the illustrated mascots.
  const pipeline = sharp(input).resize({
    width: logo ? 192 : mascot ? 768 : 1920,
    height: logo ? 192 : mascot ? 768 : 1920,
    fit: 'inside', withoutEnlargement: true,
  });
  const output = await (logo ? pipeline.png() : pipeline.webp({
    quality: 84, alphaQuality: 100, effort: 5, lossless: source.includes('pixel-'),
  })).toBuffer();
  const hash = createHash('sha256').update(output).digest('hex').slice(0, 16);
  const name = `${path.basename(source, '.png')}.${hash}.${logo ? 'png' : 'webp'}`;
  await writeFile(path.join(outputDir, name), output);
  assets[source] = `/_assets/${name}`;
  originalBytes += input.length;
  optimizedBytes += output.length;
}

await writeFile(path.join(generatedDir, 'assets.json'), JSON.stringify(assets, null, 2) + '\n');
const backgrounds = Object.entries(assets).filter(([source]) => source.startsWith('/login-bg'));
await writeFile(path.join(generatedDir, 'assets.css'), ':root {\n' + backgrounds.map(([source, url]) =>
  `  --asset-${path.basename(source, '.png')}: url("${url}");`).join('\n') + '\n}\n');
// Inline preview: the first paint never waits for a second image request.
const preview = await sharp(path.join(publicDir, 'assets/auth/conservatory.png'))
  .resize({ width: 48 }).webp({ quality: 35 }).toBuffer();
const mobilePreview = await sharp(path.join(publicDir, 'assets/auth/conservatory-mobile-v2.png'))
  .resize({ width: 32 }).webp({ quality: 35 }).toBuffer();
await writeFile(path.join(generatedDir, 'login-preview.json'), JSON.stringify({
  src: 'data:image/webp;base64,' + preview.toString('base64'),
  mobileSrc: 'data:image/webp;base64,' + mobilePreview.toString('base64'),
}) + '\n');
console.log(`UI images: ${(originalBytes / 1048576).toFixed(2)} MiB → ${(optimizedBytes / 1048576).toFixed(2)} MiB (${Object.keys(assets).length} files)`);
