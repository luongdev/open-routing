export type NotifyVariant = 'info' | 'success' | 'warning' | 'destructive';
export type NotifyPosition = 'top-left' | 'top-center' | 'top-right' | 'bottom-left' | 'bottom-center' | 'bottom-right';

export interface NotifyOptions {
  message: string;
  variant?: NotifyVariant;
  duration?: number;
  position?: NotifyPosition;
}

const VARIANT_CLASS: Record<NotifyVariant, string> = {
  info: 'uk-notification-message-info',
  success: 'uk-notification-message-success',
  warning: 'uk-notification-message-warning',
  destructive: 'uk-notification-message-danger',
};

const POSITION_CLASS: Record<NotifyPosition, string> = {
  'top-left': '',
  'top-center': 'uk-notification-top-center',
  'top-right': 'uk-notification-top-right',
  'bottom-left': 'uk-notification-bottom-left',
  'bottom-center': 'uk-notification-bottom-center',
  'bottom-right': 'uk-notification-bottom-right',
};

function getContainer(position: NotifyPosition): HTMLElement {
  const posClass = POSITION_CLASS[position];
  const selector = posClass
    ? `.uk-notification.${posClass}`
    : '.uk-notification:not([class*="uk-notification-top"]):not([class*="uk-notification-bottom"])';
  let container = document.querySelector<HTMLElement>(selector);
  if (!container) {
    container = document.createElement('div');
    container.className = ['uk-notification', posClass].filter(Boolean).join(' ');
    document.body.appendChild(container);
  }
  return container;
}

export function notify(opts: NotifyOptions): void {
  const variant = opts.variant ?? 'info';
  const position = opts.position ?? 'top-right';
  const duration = opts.duration ?? 5000;

  const container = getContainer(position);

  const msg = document.createElement('div');
  msg.className = ['uk-notification-message', VARIANT_CLASS[variant]].join(' ');
  msg.setAttribute('role', 'alert');
  msg.textContent = opts.message;

  const close = document.createElement('button');
  close.className = 'uk-notification-close';
  close.setAttribute('type', 'button');
  close.setAttribute('aria-label', 'Close notification');
  close.innerHTML = '&times;';
  close.addEventListener('click', () => remove());
  msg.appendChild(close);

  container.appendChild(msg);

  const timer = duration > 0 ? setTimeout(() => remove(), duration) : null;

  function remove(): void {
    if (timer !== null) clearTimeout(timer);
    if (msg.parentNode) msg.parentNode.removeChild(msg);
    if (container.childElementCount === 0 && container.parentNode) {
      container.parentNode.removeChild(container);
    }
  }

  msg.addEventListener('click', (e) => {
    if ((e.target as Element).closest('.uk-notification-close')) return;
    remove();
  });
}

export function notifyError(err: unknown): void {
  const message =
    err instanceof Error ? err.message
    : typeof err === 'string' ? err
    : 'Something went wrong';
  notify({ message, variant: 'destructive' });
}
