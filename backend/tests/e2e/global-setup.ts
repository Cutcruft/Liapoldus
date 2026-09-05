import { spawn, type ChildProcess } from 'node:child_process'
import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const here = fileURLToPath(new URL('.', import.meta.url))
const backendRoot = new URL('../..', import.meta.url)
const bin = new URL('../../bin/liapoldus-server', import.meta.url)

export const baseURL = process.env.LIAPOLDUS_E2E_BASE_URL ?? 'http://127.0.0.1:8080'
export const clientURL = process.env.LIAPOLDUS_E2E_CLIENT_URL ?? 'http://127.0.0.1:18080'

let server: ChildProcess | undefined

export async function setup(): Promise<void> {
  if (!existsSync(bin)) {
    const build = spawn('go', ['build', '-o', bin.pathname, './cmd/server'], {
      cwd: fileURLToPath(backendRoot),
      stdio: 'inherit',
    })
    const code = await new Promise<number | null>((resolve) => build.on('close', resolve))
    if (code !== 0) {
      throw new Error(`failed to build liapoldus-server (exit ${code})`)
    }
  }

  const addr = new URL(baseURL)
  server = spawn(bin.pathname, [], {
    env: {
      ...process.env,
      LIAPOLDUS_STORAGE: 'memory',
      LIAPOLDUS_ADMIN_ADDR: addr.host,
      LIAPOLDUS_CLIENT_ADDR: '127.0.0.1:18080',
      LIAPOLDUS_ASSET_DIR: './data/e2e-assets',
    },
    stdio: 'inherit',
  })
  server.on('error', (err) => {
    throw err
  })

  const deadline = Date.now() + 15_000
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${baseURL}/healthz`)
      if (res.ok) {
        return
      }
    } catch {
      // server not listening yet
    }
    await new Promise((resolve) => setTimeout(resolve, 200))
  }
  throw new Error(`server did not become ready at ${baseURL}`)
}

export async function teardown(): Promise<void> {
  if (!server) {
    return
  }
  server.kill('SIGTERM')
  await new Promise((resolve) => server?.on('close', resolve))
}

export { here as __dirname }