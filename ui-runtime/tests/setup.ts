import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// React 18 + vitest: включить act-окружение, сброс DOM после каждого теста.
(globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true;

afterEach(() => {
  cleanup();
});