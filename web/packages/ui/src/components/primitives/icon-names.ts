export const ICON_NAMES = [
  'home', 'users', 'calendar', 'file-text', 'pill', 'flask-conical', 'syringe', 'video',
  'bed', 'user-cog', 'list-checks', 'activity', 'plus', 'pencil', 'trash-2', 'x', 'check',
  'chevron-down', 'chevron-left', 'chevron-right', 'more-vertical',
] as const;

export type IconName = typeof ICON_NAMES[number];
