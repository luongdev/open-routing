import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import './playground-screens-route.js';
import { SCREENS } from './playground-screens.js';

describe('OrPlaygroundScreensRoute', () => {
  let el: HTMLElement;

  beforeEach(() => {
    // Ensure pathname starts at /playground/screens for tests
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      pathname: '/playground/screens',
    } as Location);
    el = document.createElement('or-playground-screens-route');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders side-nav with all 21 screens listed', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const items = sr.querySelectorAll('.screen-item');
    expect(items.length).toBe(SCREENS.length);
    expect(SCREENS.length).toBe(21);
  });

  it('groups screens into 7 labeled groups', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const groupLabels = sr.querySelectorAll('.group-label');
    expect(groupLabels.length).toBe(7);
    const labels = Array.from(groupLabels).map(g => g.textContent?.trim());
    expect(labels).toContain('Agents');
    expect(labels).toContain('Operations');
  });

  it('default state has agents-list screen active', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const active = sr.querySelector('.screen-item--active');
    expect(active?.textContent?.trim()).toBe('Agent List');
  });

  it('clicking a nav item updates _active state', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    // Stub history.pushState + location so popstate handler re-reads 'skills-list'
    vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      pathname: '/playground/screens/skills-list',
    } as Location);
    const items = sr.querySelectorAll('.screen-item');
    const skillListBtn = Array.from(items).find(
      btn => btn.textContent?.includes('Skill List')
    ) as HTMLButtonElement | undefined;
    expect(skillListBtn).toBeTruthy();
    skillListBtn!.click();
    await (el as any).updateComplete;
    const active = sr.querySelector('.screen-item--active');
    expect(active?.textContent?.trim()).toBe('Skill List');
  });

  it('renders topbar with brand title and back link', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const title = sr.querySelector('.topbar-title');
    expect(title?.textContent).toContain('Design System Demo');
    const back = sr.querySelector('.topbar-back');
    expect(back).toBeTruthy();
    expect(back?.getAttribute('href')).toBe('/playground');
  });
});
