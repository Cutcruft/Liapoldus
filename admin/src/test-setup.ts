export {};

const w = globalThis as unknown as {
  ResizeObserver?: unknown;
  PointerEvent?: unknown;
  matchMedia?: unknown;
};

if (typeof w.ResizeObserver === 'undefined') {
  class ResizeObserverStub {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  }
  w.ResizeObserver = ResizeObserverStub;
}

if (typeof w.PointerEvent === 'undefined') {
  w.PointerEvent = globalThis.MouseEvent;
}

if (typeof Element.prototype.hasPointerCapture !== 'function') {
  Element.prototype.hasPointerCapture = () => false;
  Element.prototype.setPointerCapture = () => {};
  Element.prototype.releasePointerCapture = () => {};
}

if (typeof Element.prototype.scrollIntoView !== 'function') {
  Element.prototype.scrollIntoView = () => {};
}

if (typeof w.matchMedia !== 'function') {
  w.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList;
}