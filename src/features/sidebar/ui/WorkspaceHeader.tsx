import {
  IconChevronDown,
  IconChevronsLeft,
  IconChevronsRight,
  IconDeviceDesktop,
  IconLanguage,
  IconMoon,
  IconSettings,
  IconSun,
} from "@tabler/icons-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { SettingsDialog } from "@/features/settings";
import { WorkspaceSwitcher } from "@/features/workspace";
import { useSession } from "@/shared/api/use-session";
import { LOCALES, setLocale, type Locale } from "@/shared/lib/i18n";
import { useUi } from "@/shared/lib/ui-store";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverSeparator } from "@/shared/ui/popover";
import { LogoutButton } from "./LogoutButton";

/**
 * Шапка сайдбара: рабочее пространство, профиль и выход.
 *
 * Подписи берутся из ответа логина — он уже содержит имя пользователя,
 * роль и название проекта. Отдельного запроса за профилем нет.
 */
export function WorkspaceHeader({ floating = false }: { floating?: boolean }) {
  /*
   * Окно настроек живёт здесь, а не в поповере: поповер закрывается
   * щелчком по своей же кнопке, и окно исчезло бы вместе с ним.
   */
  const [settingsOpen, setSettingsOpen] = useState(false);
  const { t } = useTranslation();
  const profile = useSession().getProfile();

  const title = profile?.company || t("app.name");
  const letter = (title[0] ?? "U").toUpperCase();

  return (
    <>
    <Popover
      trigger={({ open, toggle }) => (
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={toggle}
            className={`flex min-w-0 flex-1 items-center gap-2.5 rounded-md p-1.5 text-left transition-colors hover:bg-surface-hover ${
              open ? "bg-surface-hover" : ""
            }`}
          >
            <Avatar letter={letter} />

            <span className="flex min-w-0 flex-1 flex-col leading-tight">
              {profile?.name && (
                <span className="truncate text-xs text-fg-muted">{profile.name}</span>
              )}
              <span className="truncate text-sm font-semibold text-fg">{title}</span>
            </span>

            <Icon
              as={IconChevronDown}
              size={14}
              className={`shrink-0 text-fg-subtle transition-opacity group-hover/aside:opacity-100 ${
                open ? "opacity-100" : "opacity-0"
              }`}
            />
          </button>

          <CollapseButton floating={floating} />
        </div>
      )}
    >
        {(close) => (
          <div className="w-64">
            <div className="flex items-center gap-2.5 p-2">
              <Avatar letter={(profile?.name?.[0] ?? letter).toUpperCase()} size="lg" />
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-fg">
                  {profile?.name || t("workspace.noName")}
                </p>
                {profile?.role && (
                  <p className="truncate text-xs tracking-wide text-fg-muted uppercase">
                    {profile.role}
                  </p>
                )}
              </div>
            </div>

            <div className="flex gap-1 px-1 pb-1">
              {/* Настройки открываются окном, а не страницей: человек
                  возвращается туда же, откуда пришёл. */}
              <button
                type="button"
                onClick={() => {
                  close();
                  setSettingsOpen(true);
                }}
                className="flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border border-border text-sm text-fg transition-colors hover:bg-surface-hover"
              >
                <Icon as={IconSettings} size={14} />
                {t("workspace.settings")}
              </button>

              <LanguageButton />
            </div>

            <div className="px-1 pb-1">
              <ThemeSwitch />
            </div>

            <PopoverSeparator />

            <WorkspaceSwitcher current={title} onSwitched={close} />

            <PopoverSeparator />

            <LogoutButton onDone={close} />
        </div>
      )}
    </Popover>

    {settingsOpen && <SettingsDialog onClose={() => setSettingsOpen(false)} />}
    </>
  );
}

/**
 * Кнопка сворачивания — в шапке, рядом с переключателем рабочего
 * пространства, и появляется только при наведении на сайдбар: место
 * она занимает всегда, а нужна редко.
 */
function CollapseButton({ floating }: { floating: boolean }) {
  const { t } = useTranslation();
  const { toggleSidebar } = useUi();
  // У всплывающего сайдбара та же кнопка делает обратное: закрепляет его.
  const label = t(floating ? "sidebar.open" : "sidebar.collapse");

  return (
    <button
      type="button"
      onClick={toggleSidebar}
      aria-label={label}
      title={label}
      className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle opacity-0 transition-opacity group-hover/aside:opacity-100 hover:bg-surface-hover hover:text-fg focus-visible:opacity-100"
    >
      <Icon as={floating ? IconChevronsRight : IconChevronsLeft} size={16} />
    </button>
  );
}

/**
 * Тема живёт здесь, а не на отдельной странице настроек: её меняют
 * по настроению, а не один раз при заведении аккаунта.
 *
 * «Системная» — не то же самое, что светлая или тёмная: она следует
 * за настройкой ОС, поэтому это третий вариант, а не отсутствие выбора.
 */
function ThemeSwitch() {
  const { t } = useTranslation();
  const { theme, setTheme } = useUi();

  const options = [
    { value: "light", icon: IconSun },
    { value: "dark", icon: IconMoon },
    { value: "system", icon: IconDeviceDesktop },
  ] as const;

  return (
    <div className="flex gap-1 rounded-md border border-border p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => setTheme(option.value)}
          aria-label={t(`theme.${option.value}`)}
          title={t(`theme.${option.value}`)}
          aria-pressed={theme === option.value}
          className={`flex h-7 flex-1 items-center justify-center rounded-sm transition-colors ${
            theme === option.value
              ? "bg-surface-active text-fg"
              : "text-fg-subtle hover:bg-surface-hover hover:text-fg"
          }`}
        >
          <Icon as={option.icon} size={15} />
        </button>
      ))}
    </div>
  );
}

/** Язык переключается по кругу — трёх языков мало для отдельного меню. */
function LanguageButton() {
  const { i18n } = useTranslation();
  const current = (LOCALES as readonly string[]).includes(i18n.language)
    ? (i18n.language as Locale)
    : LOCALES[0];

  const next = LOCALES[(LOCALES.indexOf(current) + 1) % LOCALES.length]!;

  return (
    <button
      type="button"
      onClick={() => setLocale(next)}
      className="flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border border-border text-sm text-fg uppercase transition-colors hover:bg-surface-hover"
    >
      {current}
      <Icon as={IconLanguage} size={14} />
    </button>
  );
}

function Avatar({
  letter,
  size = "md",
  tone = "accent",
}: {
  letter: string;
  size?: "md" | "lg";
  tone?: "accent" | "muted";
}) {
  return (
    <span
      className={`grid shrink-0 place-items-center rounded-md font-semibold ${
        size === "lg" ? "size-9 text-base" : "size-7 text-xs"
      } ${tone === "accent" ? "bg-accent-solid text-accent-fg" : "bg-surface-active text-fg-muted"}`}
    >
      {letter}
    </span>
  );
}
