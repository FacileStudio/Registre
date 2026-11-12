import { registerIcons, type IconData } from '@facile/muse';
import data from './icons-data.json';

/** DISCORD_ICON is the key `<Icon icon={DISCORD_ICON} />` and `<Button icon=…>` take. */
export const DISCORD_ICON = 'brand:discord';

/** GITHUB_ICON is the key `<Icon icon={GITHUB_ICON} />` and `<Button icon=…>` take. */
export const GITHUB_ICON = 'brand:github';

/** REGISTRE_ICON is Registre's catalog glyph, the key `<LandingHeader icon=…>` takes. */
export const REGISTRE_ICON = 'solar:key-square-bold-duotone';

export const ICON_DATA = data as Record<string, IconData>;

registerIcons(ICON_DATA);
