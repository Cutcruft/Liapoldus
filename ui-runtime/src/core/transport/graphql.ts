import { TransportError } from '../../errors';
import type { ResolvedProvider } from '../registry';
import type { FetchLike, Transport, TransportRequest, TransportResponse } from './transport';

const DEFAULT_TIMEOUT_MS = 30_000;

/** GraphQL-транспорт: POST { query, variables } на baseUrl. */
export class GraphqlTransport implements Transport {
  private readonly fetcher: FetchLike;

  constructor(private readonly provider: ResolvedProvider, env?: { fetch?: FetchLike }) {
    this.fetcher = env?.fetch ?? (globalThis.fetch as FetchLike);
    if (!this.fetcher) throw new Error('GraphqlTransport требует fetch');
  }

  buildUrl(): string {
    const base = this.provider.baseUrl ?? '/graphql';
    return base;
  }

  async request(req: TransportRequest): Promise<TransportResponse> {
    const query = req.gql;
    if (!query) {
      throw new TransportError(`GraphQL-запрос для '${req.operation.id}' требует gql-строку`);
    }
    const url = this.buildUrl();
    const timeoutMs = req.timeoutMs ?? this.provider.defaults?.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    const controller = new AbortController();
    let timedOut = false;
    const timer = timeoutMs > 0 ? setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs) : undefined;

    const abortPromise = new Promise<never>((_, reject) => {
      controller.signal.addEventListener('abort', () => {
        reject(
          new TransportError(
            timedOut ? `Таймаут GraphQL-запроса (${timeoutMs}мс)` : 'GraphQL-запрос прерван',
          ),
        );
      });
    });

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.provider.defaults?.headers) Object.assign(headers, this.provider.defaults.headers);
    if (req.headers) Object.assign(headers, req.headers);

    try {
      const body = JSON.stringify({ query, variables: req.input ?? {} });
      const fetchPromise = this.fetcher(url, {
        method: 'POST',
        headers,
        body,
        signal: controller.signal,
      });
      const res = await Promise.race([fetchPromise, abortPromise]);
      if (!res.ok) {
        throw new TransportError(`HTTP ${res.status} для GraphQL`, { status: res.status, httpError: true });
      }
      const payload = (await res.json().catch(() => null)) as { data?: unknown; errors?: unknown[] } | null;
      if (payload && Array.isArray(payload.errors) && payload.errors.length > 0) {
        throw new TransportError('GraphQL вернул ошибки', { graphqlErrors: payload.errors });
      }
      return { status: res.status, ok: true, body: payload?.data ?? payload };
    } catch (err) {
      if (err instanceof TransportError) throw err;
      throw new TransportError(`Ошибка GraphQL: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      if (timer) clearTimeout(timer);
    }
  }
}