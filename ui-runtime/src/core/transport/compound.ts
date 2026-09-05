import type { Transport, TransportRequest } from './transport';
import type { TransportFactory } from './factory';

/** Роутит запросы к транспорту провайдера запроса (по provider.id через фабрику). */
export class CompoundTransport implements Transport {
  constructor(private readonly factory: TransportFactory) {}

  request(req: TransportRequest): Promise<import('./transport').TransportResponse> {
    return this.factory.create(req.provider).request(req);
  }

  subscribe(
    req: TransportRequest,
    onData: (data: unknown) => void,
    onError?: (e: Error) => void,
  ): () => void {
    const transport = this.factory.create(req.provider);
    if (typeof transport.subscribe !== 'function') {
      throw new Error(`Транспорт провайдера '${req.provider.id}' не поддерживает push-канал`);
    }
    return transport.subscribe(req, onData, onError);
  }

  close(): void {
    this.factory.clear();
  }
}