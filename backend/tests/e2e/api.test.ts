import { describe, it, expect } from 'vitest'
import request from 'supertest'
import { baseURL } from './global-setup.js'

const api = request(baseURL)

const textSchema = { type: 'object', required: ['text'], properties: { text: { type: 'string' } } }
const containerSchema = { type: 'object', properties: { gap: { type: 'number' } } }

function root(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    instanceId: 'root',
    definitionId: 'container',
    props: {},
    children: [{ instanceId: 'title', definitionId: 'text', props: { text: 'Hello' } }],
    ...overrides,
  }
}

// Seed a site plus the components its page trees reference, returning the body
// of the page create/update call made by the caller through `act`.
async function withSeededSite(
  name: string,
  slug: string,
  act: (site: { id: string }) => Promise<request.Response>,
): Promise<request.Response> {
  const createSite = await api
    .post('/api/sites')
    .set('authorization', 'Bearer e2e-admin-token')
    .send({ name, slug })
  expect(createSite.status).toBe(201)
  const site = createSite.body

  for (const def of [
    { id: 'container', name: 'Container', source: 'export const Container = () => null', schema: containerSchema, metadata: { label: 'Container' } },
    { id: 'text', name: 'Text', source: 'export const Text = () => null', schema: textSchema, metadata: { label: 'Text' } },
  ]) {
    const defRes = await api
      .post(`/api/sites/${site.id}/components`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send(def)
    expect(defRes.status).toBe(201)
  }
  return act(site)
}

describe('REST API', () => {
  it('reports health', async () => {
    const res = await api.get('/healthz')
    expect(res.status).toBe(200)
    expect(res.body).toEqual({ status: 'ok' })
  })

  it('rejects creating a page for a missing site', async () => {
    const res = await api
      .post('/api/sites/site_missing/pages')
      .set('authorization', 'Bearer e2e-admin-token')
      .send({ name: 'Home', slug: 'home', root: root() })
    expect(res.status).toBe(404)
  })

  it('runs the full site, page and snapshot flow', async () => {
    const res = await withSeededSite('Demo', 'demo', async (site) =>
      api
        .post(`/api/sites/${site.id}/pages`)
        .set('authorization', 'Bearer e2e-admin-token')
        .send({ name: 'Home', slug: 'home', root: root() }),
    )
    expect(res.status).toBe(201)
    const page = res.body
    expect(page.id).toMatch(/^page_/)
    expect(page).toMatchObject({ name: 'Home', slug: 'home', version: 1 })

    const pageSite = (page.siteId as string) || page.siteId
    const siteID = page.siteId

    const listPages = await api
      .get(`/api/sites/${siteID}/pages`)
      .set('authorization', 'Bearer e2e-admin-token')
    expect(listPages.status).toBe(200)
    expect(listPages.body).toHaveLength(1)

    const updateTree = await api
      .put(`/api/pages/${page.id}/tree`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send({
        root: root({ children: [{ instanceId: 'updated', definitionId: 'text', props: { text: 'Updated' } }] }),
      })
    expect(updateTree.status).toBe(200)
    expect(updateTree.body.version).toBe(2)

    const versions = await api
      .get(`/api/pages/${page.id}/versions`)
      .set('authorization', 'Bearer e2e-admin-token')
    expect(versions.status).toBe(200)
    expect(versions.body).toHaveLength(2)
    expect(versions.body[0].number).toBe(1)
    expect(versions.body[1].number).toBe(2)

    const createSnapshot = await api
      .post(`/api/sites/${siteID}/snapshots`)
      .set('authorization', 'Bearer e2e-admin-token')
      .send({ name: 'Release 1' })
    expect(createSnapshot.status).toBe(201)
    const snapshot = createSnapshot.body
    expect(snapshot.id).toMatch(/^snapshot_/)
    expect(snapshot.pages).toHaveLength(1)
    expect(snapshot.pages[0]).toMatchObject({ pageId: page.id, version: 2 })

    const getSnapshot = await api
      .get(`/api/snapshots/${snapshot.id}`)
      .set('authorization', 'Bearer e2e-admin-token')
    expect(getSnapshot.status).toBe(200)
    expect(getSnapshot.body).toEqual(snapshot)
    expect(pageSite).toBeTruthy()
  })

  it('defines a component and lists its versions', async () => {
    const res = await withSeededSite('Comp', 'comp', async (site) =>
      api
        .post(`/api/sites/${site.id}/components`)
        .set('authorization', 'Bearer e2e-admin-token')
        .send({
          id: 'hero',
          name: 'Hero',
          source: 'export const Hero = () => null',
          schema: { type: 'object' },
          metadata: { label: 'Hero' },
        }),
    )
    expect(res.status).toBe(201)
    expect(res.body.id).toMatch(/^[0-9a-f]{40}$/)
    expect(res.body.definitionId).toBe('hero')
    expect(res.body.siteId).toMatch(/^site_/)

    const list = await api
      .get(`/api/sites/${res.body.siteId}/components`)
      .set('authorization', 'Bearer e2e-admin-token')
    expect(list.status).toBe(200)
    const ids = list.body.map((c: { id: string }) => c.id)
    expect(ids).toContain('hero')
  })

  it('rejects invalid page tree referencing an unknown definition', async () => {
    const res = await withSeededSite('Bad', 'bad', (site) =>
      api
        .post(`/api/sites/${site.id}/pages`)
        .set('authorization', 'Bearer e2e-admin-token')
        .send({ name: 'Broken', slug: 'broken', root: { instanceId: 'root', definitionId: 'unknown' } }),
    )
    expect(res.status).toBe(404)
  })
})