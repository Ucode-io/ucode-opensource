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
import { SettingsDialog, useProject } from "@/features/settings";
import { WorkspaceSwitcher } from "@/features/workspace";
import { useSession } from "@/shared/api/use-session";
import { LOCALES, setLocale, type Locale } from "@/shared/lib/i18n";
import { useUi } from "@/shared/lib/ui-store";
import { BrandMark } from "@/shared/ui/brand-mark";
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
  /*
   * Логотип проекта — из настроек проекта. Запрос уже сделан там же
   * и живёт в кэше пять минут; своего здесь не появляется.
   */
  const { project } = useProject();
  const logo = project?.logo ?? "";

  return (
    <>
    <Popover
      trigger={({ open, toggle }) => (
        /*
         * Подсветка — на ряду целиком, а не на кнопке поповера: кнопка
         * сворачивания стоит ВНУТРИ этой подсветки, и ряд обязан
         * подсвечиваться, когда курсор на ней. Кнопки при этом две,
         * а не одна: вложенных <button> не бывает, да и действия у них
         * разные — открыть меню и убрать панель.
         */
        <div
          className={`flex items-center gap-1 rounded-md p-1.5 transition-colors hover:bg-surface-hover ${
            open ? "bg-surface-hover" : ""
          }`}
        >
          <button
            type="button"
            onClick={toggle}
            className="flex min-w-0 flex-1 items-center gap-2.5 text-left"
          >
            <Avatar
              letter={letter}
              brand={!profile?.company}
              {...(logo ? { image: logo } : {})}
            />

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
              <Avatar
                letter={(profile?.name?.[0] ?? letter).toUpperCase()}
                size="lg"
                {...(profile?.photo ? { image: profile.photo } : {})}
              />
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
                className={POPOVER_BUTTON}
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
 * Кнопка в поповере профиля: 28px, а не 32px как обычная кнопка формы.
 *
 * Поповер узкий, и обведённых рамкой кнопок в нём подряд три. В полный
 * рост они спорили бы со строками списка под собой — а это действия
 * рядом, не главное в меню. 28px — тот же размер, что у кнопок-иконок
 * над таблицей (ToolButton): «компактное управление» в системе одно.
 */
const POPOVER_BUTTON =
  "flex h-7 flex-1 items-center justify-center gap-1.5 rounded-md border border-border text-sm text-fg transition-colors hover:bg-surface-hover";

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
      /* Наведение красит `surface-active`, а не `surface-hover`: кнопка
         лежит внутри подсвеченного ряда, и второй такой же заливкой
         она бы на нём не проступила. */
      className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle opacity-0 transition-opacity group-hover/aside:opacity-100 hover:bg-surface-active hover:text-fg focus-visible:opacity-100"
    >
      {/* Пара зеркальная: убрать панель — стрелки влево, вернуть —
          вправо. Это про НАПРАВЛЕНИЕ, и читается без словаря; коробка
          с панелью внутри на 16px спорила бы сама с собой — рисует
          сайдбар там, где его сейчас нет. Тот же значок стоит на кнопке
          в шапке контента: действие одно (и ключ подписи один). */}
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
    /* Высота — как у соседних кнопок поповера (28px), считая рамку
       и внутренний зазор: переключатель стоит с ними в одном столбце,
       и на 34px он читался бы как что-то более важное. */
    <div className="flex h-7 gap-1 rounded-md border border-border p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => setTheme(option.value)}
          aria-label={t(`theme.${option.value}`)}
          title={t(`theme.${option.value}`)}
          aria-pressed={theme === option.value}
          className={`flex h-full flex-1 items-center justify-center rounded-sm transition-colors ${
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
      className={`${POPOVER_BUTTON} uppercase`}
    >
      {current}
      <Icon as={IconLanguage} size={14} />
    </button>
  );
}

/**
 * Знак рабочего пространства: картинка, если её загрузили, иначе буква.
 *
 * Картинка — логотип проекта у компании и фотография у человека; обе
 * задаются в настройках. Буква остаётся запасным вариантом: логотип
 * есть далеко не у каждого проекта, и пустой квадрат хуже буквы.
 *
 * `brand` — частный случай буквы: рабочее пространство без своей
 * компании и своего логотипа показывает знак ucode вместо буквы «U»,
 * а не собственный бренд-цвет для чужого проекта.
 */
function Avatar({
  letter,
  image,
  brand = false,
  size = "md",
  tone = "accent",
}: {
  letter: string;
  image?: string | undefined;
  brand?: boolean;
  size?: "md" | "lg";
  tone?: "accent" | "muted";
}) {
  const px = size === "lg" ? 36 : 28;
  const box = size === "lg" ? "size-9 text-base" : "size-7 text-xs";

  if (image) {
    return (
      <span className={`grid shrink-0 place-items-center overflow-hidden rounded-md ${box}`}>
        <img src={image} alt="" className="size-full object-cover" />
      </span>
    );
  }

  if (brand) {
    return (
      <span className={`grid shrink-0 place-items-center rounded-md ${box}`}>
        <BrandMark size={px} />
      </span>
    );
  }

  return (
    <span
      className={`grid shrink-0 place-items-center rounded-md font-semibold ${box} ${
        tone === "accent" ? "bg-accent-solid text-accent-fg" : "bg-surface-active text-fg-muted"
      }`}
    >
      {letter}
    </span>
  );
}
