export type State = { n: number };

const state: State = { n: 1 };

export const Store = {
  version: () => `v-${state.n}`,
};