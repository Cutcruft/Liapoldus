import { describe, it, expect } from 'vitest'
import request from 'supertest'
import { baseURL } from './global-setup.js'

const api = request(baseURL)

function root(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'root',
    type: 'Container',
    children: [{ id: 'title', type: 'Text', props: { text: 'Hello' } }],
    ...overrides,
  }
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
      .send({ name: 'Home', slug: 'home', root: root() })
    expect(res.status).toBe(404)
  })

  it('runs the full site, page and snapshot flow', async () => {
    const createSite = await api.post('/api/sites').send({ name: 'Demo', slug: 'demo' })
    expect(createSite.status).toBe(201)
    const site = createSite.body
    expect(site.id).toMatch(/^site_/)
    expect(site).toMatchObject({ name: 'Demo', slug: 'demo' })

    const getSite = await api.get(`/api/sites/${site.id}`)
    expect(getSite.status).toBe(200)
    expect(getSite.body).toEqual(site)

    const createPage = await api
      .post(`/api/sites/${site.id}/pages`)
      .send({ name: 'Home', slug: 'home', root: root() })
    expect(createPage.status).toBe(201)
    const page = createPage.body
    expect(page.id).toMatch(/^page_/)
    expect(page).toMatchObject({ siteId: site.id, name: 'Home', slug: 'home', version: 1 })

    const listPages = await api.get(`/api/sites/${site.id}/pages`)
    expect(listPages.status).toBe(200)
    expect(listPages.body).toHaveLength(1)

    const updateTree = await api
      .put(`/api/pages/${page.id}/tree`)
      .send({ root: root({ children: [{ id: 'updated', type: 'Button' }] }) })
    expect(updateTree.status).toBe(200)
    expect(updateTree.body.version).toBe(2)

    const versions = await api.get(`/api/pages/${page.id}/versions`)
    expect(versions.status).toBe(200)
    expect(versions.body).toHaveLength(2)
    expect(versions.body[0].number).toBe(1)
    expect(versions.body[1].number).toBe(2)

    const createSnapshot = await api
      .post(`/api/sites/${site.id}/snapshots`)
      .send({ name: 'Release 1' })
    expect(createSnapshot.status).toBe(201)
    const snapshot = createSnapshot.body
    expect(snapshot.id).toMatch(/^snapshot_/)
    expect(snapshot.pages).toHaveLength(1)
    expect(snapshot.pages[0]).toMatchObject({ pageId: page.id, version: 2 })

    const getSnapshot = await api.get(`/api/snapshots/${snapshot.id}`)
    expect(getSnapshot.status).toBe(200)
    expect(getSnapshot.body).toEqual(snapshot)
  })

  it('rejects invalid page tree', async () => {
    const createSite = await api.post('/api/sites').send({ name: 'Bad', slug: 'bad' })
    const site = createSite.body

    const res = await api
      .post(`/api/sites/${site.id}/pages`)
      .send({ name: 'Broken', slug: 'broken', root: { id: 'root', type: 'Unknown' } })
    expect(res.status).toBe(400)
  })
})