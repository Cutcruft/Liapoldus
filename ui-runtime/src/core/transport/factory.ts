import type { ResolvedProvider } from '../registry';
import { GraphqlTransport } from './graphql';
import { HttpTransport } from './http';
import type { Transport, TransportEnv } from './transport';
import { SseTransport, WebSocketTransport } from './ws-sse';
import { pickEventSource, pickWebSocket } from './transport';

/** Фабрика транспортов по протоколу провайдера с кэшем на provider.id. */
export class TransportFactory {
  private readonly cache = new Map<string, Transport>();

  constructor(private readonly env?: TransportEnv) {}

  create(provider: ResolvedProvider): Transport {
    const cached = this.cache.get(provider.id);
    if (cached) return cached;

    let transport: Transport;
    switch (provider.protocol) {
      case 'http':
        transport = new HttpTransport(provider, { fetch: this.env?.fetch });
        break;
      case 'graphql':
        transport = new GraphqlTransport(provider, { fetch: this.env?.fetch });
        break;
      case 'ws':
        transport = new WebSocketTransport(provider, pickWebSocket(this.env));
        break;
      case 'sse':
        transport = new SseTransport(pickEventSource(this.env));
        break;
      default: {
        const exhaustive: never = provider.protocol;
        throw new Error(`Неизвестный протокол транспорта: ${String(exhaustive)}`);
      }
    }
    this.cache.set(provider.id, transport);
    return transport;
  }

  clear(): void {
    for (const t of this.cache.values()) t.close?.();
    this.cache.clear();
  }
}