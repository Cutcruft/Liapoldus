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

// ProseMirror/Tiptap в jsdom: Range.getBoundingClientRect/getClientRects не реализованы.
if (typeof Range !== 'undefined') {
  if (typeof Range.prototype.getBoundingClientRect !== 'function') {
    Range.prototype.getBoundingClientRect = () => ({
      x: 0, y: 0, width: 0, height: 0, top: 0, right: 0, bottom: 0, left: 0,
      toJSON: () => ({}),
    });
  }
  if (typeof Range.prototype.getClientRects !== 'function') {
    Range.prototype.getClientRects = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] });
  }
}
const textProto = Text.prototype as unknown as { getClientRects?: () => unknown };
if (typeof textProto.getClientRects !== 'function') {
  textProto.getClientRects = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] });
}