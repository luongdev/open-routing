// Phase 2 D-39: Lit-aware async helper. Wraps @lit/task for the common
// loading/error/data pattern that admin SPA components hit. Phase 7
// embed can opt out by importing only createApiClient + types (skip
// this module).

import { Task } from '@lit/task';
import type { ReactiveControllerHost } from '@lit/reactive-element/reactive-controller.js';

/**
 * createApiTask wraps a @lit/task Task with passthrough config. Used by
 * Lit components in apps/admin (and optionally apps/embed) to wire
 * fetch-like operations into the component lifecycle.
 *
 * @param host - the Lit ReactiveControllerHost (typically the component).
 * @param config.task - the async task body; receives the resolved args
 *   tuple plus an AbortSignal for cancellation on host disconnect.
 * @param config.args - reactive args resolver; @lit/task re-runs the
 *   task whenever the tuple changes by identity.
 */
export function createApiTask<
  TArgs extends readonly unknown[],
  TData,
>(
  host: ReactiveControllerHost,
  config: {
    task: (args: TArgs, signal: AbortSignal) => Promise<TData>;
    args: () => TArgs;
  },
): Task<TArgs, TData> {
  return new Task<TArgs, TData>(host, {
    task: (args: TArgs, options: { signal: AbortSignal }) => config.task(args, options.signal),
    args: config.args,
  });
}
