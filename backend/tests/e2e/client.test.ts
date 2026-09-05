import { describe, it, expect, beforeAll } from 'vitest'
import request from 'supertest'
import { baseURL, clientURL } from './global-setup.js'

const api = request(baseURL)
const client = request(clientURL)

let siteId: string
let formId: string

beforeAll(async () => {
  const createSite = await api.post('/api/sites').send({
    name: 'Client Demo',
    slug: 'client-demo',
    defaultLocale: 'ru',
    hosts: [`127.0.0.1`],
  })
  expect(createSite.status).toBe(201)
  siteId = createSite.body.id

  const createForm = await api.post(`/api/sites/${siteId}/forms`).send({
    name: 'contact',
    definition: { fields: [{ name: 'email', type: 'email', required: true }] },
  })
  expect(createForm.status).toBe(201)
  formId = createForm.body.id
})

describe('Client API', () => {
  it('returns merged content by locale via host resolution', async () => {
    const createContent = await api.post(`/api/sites/${siteId}/contents`).send({
      collectionId: 'col.articles',
      id: 'intro',
      fields: { title: 'Base title', description: 'Base description' },
    })
    expect(createContent.status).toBe(201)

    const putTranslation = await api
      .put(`/api/sites/${siteId}/contents/intro/translations/ru`)
      .send({ fields: { title: 'Привет' } })
    expect(putTranslation.status).toBe(200)

    const get = await client
      .get(`/api/contents/intro?locale=ru`)
      .set('Host', '127.0.0.1')
    expect(get.status).toBe(200)
    expect(get.body).toMatchObject({
      id: 'intro',
      locale: 'ru',
      collectionId: 'col.articles',
    })
    expect(get.body.fields).toMatchObject({ title: 'Привет', description: 'Base description' })
  })

  it('submits a form and validates email server-side', async () => {
    const invalid = await client
      .post(`/api/forms/${formId}/submissions`)
      .send({ values: { email: 'not-an-email' } })
    expect(invalid.status).toBe(400)

    const valid = await client
      .post(`/api/forms/${formId}/submissions`)
      .send({ values: { email: 'user@example.com' } })
    expect(valid.status).toBe(201)
    expect(valid.body).toMatchObject({ status: 'ok' })
    expect(valid.body.submissionId).toMatch(/^submission_/)
  })

  it('serves a redirect with group expansion from the edge', async () => {
    const createRoute = await api.post(`/api/sites/${siteId}/routes`).send({
      matcher: `^/products/([a-z0-9-]+)$`,
      priority: 0,
      action: { type: 'redirect', target: '/shop/$1', status: 302 },
    })
    expect(createRoute.status).toBe(201)

    const res = await client.get('/products/sneakers').set('Host', '127.0.0.1')
    expect(res.status).toBe(302)
    expect(res.headers.location).toBe('/shop/sneakers')
  })
})