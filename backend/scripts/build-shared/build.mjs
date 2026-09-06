#!/usr/bin/env node
// Bundles the shared runtime libraries into
// backend/internal/infra/build/shared/embed/<key>/<version>.js so the Go
// binary can go:embed them and serve them at /build/_shared/<key>/<version>.js.
//
// The react/react-dom versions below MUST match the Artifacts table in
// internal/infra/build/shared/shared.go; the ui-runtime version is read from
// its package.json.
//
// Run: npm ci && npm run build

import esbuild from 'esbuild'
import { mkdir, readFile, stat, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const backendRoot = resolve(scriptDir, '../..')
const outRoot = resolve(backendRoot, 'internal/infra/build/shared/embed')
const uiRuntimePkg = JSON.parse(await readFile(resolve(backendRoot, '../ui-runtime/package.json'), 'utf8'))

const REACT_VERSION = '18.3.1'
const UI_RUNTIME_VERSION = uiRuntimePkg.version

const browser = {
  bundle: true,
  format: 'esm',
  platform: 'browser',
  target: 'es2020',
  minify: true,
  jsx: 'automatic',
  logLevel: 'silent',
}

async function bundleStdout(name, outfile, external, stdin) {
  const result = await esbuild.build({ ...browser, outfile, external, stdin })
  if (result.errors.length > 0) {
    for (const err of result.errors) console.error(err.text)
    throw new Error(`shared bundle failed: ${name}`)
  }
  const bytes = (await stat(outfile)).size
  console.log(`${name.padEnd(28)} ${bytes} bytes`)
}

async function bundle(browserOptions, entryPoint, overrides, name) {
  const result = await esbuild.build({ ...browserOptions, entryPoints: [entryPoint], ...overrides })
  if (result.errors.length > 0) {
    for (const err of result.errors) console.error(err.text)
    throw new Error(`shared bundle failed: ${name}`)
  }
  const bytes = (await stat(overrides.outfile)).size
  console.log(`${name.padEnd(28)} ${bytes} bytes`)
}

async function main() {
  await mkdir(outRoot, { recursive: true })

  // react: single ESM module with the React namespace as default export (the
  // import map serves it to bare "react" imports, including `import React from
  // "react"` used by component definitions).
  await bundleStdout('react', resolve(outRoot, 'react', `${REACT_VERSION}.js`), [], {
    contents: `import React from "react";
export default React;
export * from "react";
`,
    resolveDir: scriptDir,
    sourcefile: 'react-reexport.ts',
    loader: 'ts',
  })

  // react-dom: client entry points (createRoot/hydrateRoot) merged into one
  // module so a single import-map key covers both.
  await bundleStdout('react-dom', resolve(outRoot, 'react-dom', `${REACT_VERSION}.js`), ['react'], {
    contents: `import ReactDOM from "react-dom";
import { createRoot, hydrateRoot } from "react-dom/client";
export default ReactDOM;
export { createRoot, hydrateRoot };
export * from "react-dom";
`,
    resolveDir: scriptDir,
    sourcefile: 'react-dom-reexport.ts',
    loader: 'ts',
  })

  // react/jsx-runtime: what esbuild's automatic JSX transform imports.
  await bundleStdout('react/jsx-runtime', resolve(outRoot, 'react', 'jsx-runtime', `${REACT_VERSION}.js`), ['react'], {
    contents: `import { Fragment, jsx, jsxs } from "react/jsx-runtime";
export { Fragment, jsx, jsxs };
`,
    resolveDir: scriptDir,
    sourcefile: 'jsx-runtime-reexport.ts',
    loader: 'ts',
  })

  // ui-runtime: the site runtime compiled from ui-runtime/src. react and
  // react-dom stay external so the whole React surface is shared with the
  // site bundle.
  await bundle(browser, resolve(backendRoot, '../ui-runtime/src/index.ts'), {
    outfile: resolve(outRoot, '@liapoldus', 'ui-runtime', `${UI_RUNTIME_VERSION}.js`),
    external: ['react', 'react-dom'],
  }, '@liapoldus/ui-runtime')

  // Static smoke checks: each output must parse as ESM and carry its expected
  // public surface.
  for (const [name, file, markers] of [
    ['react', resolve(outRoot, 'react', `${REACT_VERSION}.js`), ['createElement']],
    ['react-dom', resolve(outRoot, 'react-dom', `${REACT_VERSION}.js`), ['createRoot']],
    ['react/jsx-runtime', resolve(outRoot, 'react', 'jsx-runtime', `${REACT_VERSION}.js`), ['jsx']],
    ['@liapoldus/ui-runtime', resolve(outRoot, '@liapoldus', 'ui-runtime', `${UI_RUNTIME_VERSION}.js`), ['boot', 'RuntimeRegistry']],
  ]) {
    const source = await readFile(file, 'utf8')
    try {
      await esbuild.transform(source, { sourcemap: false, format: 'esm' })
    } catch (err) {
      throw new Error(`invalid ESM output: ${name}: ${err.message}`)
    }
    for (const marker of markers) {
      if (!source.includes(marker)) throw new Error(`${name} is missing ${marker}`)
    }
    console.log(`${name.padEnd(28)} e2e-smoke ok`)
  }

  console.log('shared bundles written to', outRoot)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})