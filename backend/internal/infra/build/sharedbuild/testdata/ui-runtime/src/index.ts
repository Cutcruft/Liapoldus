import { createElement } from "react";
import { Store } from "./store";
import { view } from "./view";

export { Store };
export type { State } from "./store";
export { view };

export function boot(): string {
  return `boot:${Store.version()}`;
}