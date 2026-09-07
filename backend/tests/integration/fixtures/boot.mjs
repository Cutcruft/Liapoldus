#!/usr/bin/env node
// Автономный boot-скрипт (Этап 4): boot()s real ui-runtime core against a
// live contract and prints a JSON probe the Go integration test asserts on.
//
// Usage: node boot.mjs <baseUrl> <siteId> <environment>
//
// boot() runs headless: it needs fetch + storage injected through BootEnv and
// never touches the DOM. Production environment avoids the dev WS channel.

import { boot } from './ui-runtime-core.mjs'

const [baseUrl, siteId, environment] = process.argv.slice(2)
if (!baseUrl || !siteId || !environment) {
  console.error('usage: node boot.mjs <baseUrl> <siteId> <environment>')
  process.exit(2)
}

let runtime
try {
  runtime = await boot(siteId, environment, {
    baseUrl,
    env: {
      fetch: (...args) => fetch(...args),
      storage: { getItem: () => null, setItem: () => {} },
      navigatorLanguage: 'en',
    },
  })
} catch (err) {
  console.error('boot failed:', err)
  process.exit(1)
}

const home = runtime.router.match('/')
const page = runtime.tree.elements
process.stdout.write(
  JSON.stringify({
    siteId: runtime.siteId,
    environment: runtime.environment,
    ready: runtime.ready,
    locale: runtime.i18n.getLocale(),
    hasContentOp: runtime.registry.hasOperation('content.get'),
    homePageId: home?.route.action.type === 'renderPage' ? home.route.action.pageId : null,
    pageId: runtime.store.getState().tree?.pageId ?? null,
    pageElementCount: page?.length ?? 0,
    firstElementId: page?.[0]?.id ?? null,
    firstElementComponentId: page?.[0]?.componentId ?? null,
  }),
)
process.exit(0)