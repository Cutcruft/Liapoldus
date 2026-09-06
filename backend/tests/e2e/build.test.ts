import { describe, it, expect } from 'vitest'
import request from 'supertest'
import { baseURL, clientURL } from './global-setup.js'

const api = request(baseURL)
const client = request(clientURL)

const textSchema = { type: 'object', required: ['text'], properties: { text: { type: 'string' } } }
const containerSchema = { type: 'object', properties: { gap: { type: 'number' } } }

// Definitions must export a default so the materializer's entry.tsx can import
// them (unlike api.test.ts which only checks registry storage).
const textSource = `
import React from "react";
export default (props) => React.createElement("span", null, props?.text ?? "");
`
const containerSource = `
import React from "react";
export default (props) => React.createElement("div", { style: { gap: props?.gap } }, props?.children ?? null);
`

async function seedSite() {
  const slug = `build-site-${Math.random().toString(36).slice(2, 8)}`
  const createSite = await api
    .post('/api/sites')
    .set('authorization', 'Bearer e2e-admin-token')
    .send({ name: 'Build site', slug })
  expect(createSite.status).toBe(201)
  const site = createSite.body

  for (const def of [
    { id: 'container', name: 'Container', source: containerSource, schema: containerSchema, metadata: { label: 'Container' } },
    { id: 'text', name: 'Text', source: textSource, schema: textSchema, metadata: { label: 'Text' } },
  ]) {
    const defRes = await api
      .post(`/api/sites/${site.id}/components`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send(def)
    expect(defRes.status).toBe(201)
  }

  const pageRes = await api
    .post(`/api/sites/${site.id}/pages`)
    .set('authorization', 'Bearer e2e-admin-token')
    .send({
      name: 'Home',
      slug: 'index',
      root: {
        instanceId: 'root',
        definitionId: 'container',
        props: { gap: 12 },
        children: [{ instanceId: 'title', definitionId: 'text', props: { text: 'Hello' } }],
      },
    })
  expect(pageRes.status).toBe(201)

  const snapRes = await api
    .post(`/api/sites/${site.id}/snapshots`)
    .set('authorization', 'Bearer e2e-admin-token')
    .send({ name: 'v1' })
  expect(snapRes.status).toBe(201)
  return { site, snapshot: snapRes.body }
}

describe('builds (Этап 3)', () => {
  it('publishes a snapshot synchronously and serves its bundle', async () => {
    const { site, snapshot } = await seedSite()

    const publish = await api
      .post(`/api/sites/${site.id}/builds`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send({ snapshotId: snapshot.id, environment: 'development' })
    expect(publish.status).toBe(201)
    const build = publish.body
    expect(build.id).toBeTruthy()
    expect(build.status).toBe('ready')

    // The same publication is a no-op: same build id.
    const again = await api
      .post(`/api/sites/${site.id}/builds`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send({ snapshotId: snapshot.id, environment: 'development' })
    expect(again.status).toBe(201)
    expect(again.body.id).toBe(build.id)

    // GET exposes status and a non-empty log.
    const get = await api
      .get(`/api/builds/${build.id}`)
      .set('authorization', 'Bearer e2e-admin-token')
    expect(get.status).toBe(200)
    expect(get.body.status).toBe('ready')
    expect(get.body.log.length).toBeGreaterThan(0)

    // The artifact bundle is served from the client end point.
    const bundle = await client.get(`/build/${site.id}/development/${snapshot.id}/dist/entry.js`)
    expect(bundle.status).toBe(200)
    expect(bundle.headers['content-type']).toMatch(/javascript/)
    expect(bundle.text.length).toBeGreaterThan(16)

    // A path outside the artifact directory is a 404.
    const traversal = await client.get(`/build/${site.id}/development/${snapshot.id}/../../../../etc/passwd`)
    expect([400, 404]).toContain(traversal.status)
  })

  it('rejects invalid environments and missing snapshots', async () => {
    const { site, snapshot } = await seedSite()

    const badEnv = await api
      .post(`/api/sites/${site.id}/builds`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send({ snapshotId: snapshot.id, environment: 'staging' })
    expect(badEnv.status).toBe(400)

    const missingSnap = await api
      .post(`/api/sites/${site.id}/builds`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send({ snapshotId: 'snapshot_nope', environment: 'development' })
    expect(missingSnap.status).toBe(404)
  })
})