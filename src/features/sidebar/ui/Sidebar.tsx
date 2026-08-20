import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { IconLayoutSidebarLeftExpand } from "@tabler/icons-react";
import { errorText } from "@/shared/api/client";
import { Icon } from "@/shared/ui/icon";
import { ResizeHandle } from "@/shared/ui/resize-handle";
import { SIDEBAR_MAX_WIDTH, SIDEBAR_MIN_WIDTH, useUi } from "@/shared/lib/ui-store";
import { ROOT_MENU_ID, useMenuChildren } from "../api/menus";
import type { MenuNode } from "../model/types";
import { AddMenuButton } from "./AddMenuButton";
import { MenuDndProvider } from "./dnd-context";
import { MenuLevel } from "./MenuLevel";
import { WorkspaceHeader } from "./WorkspaceHeader";

/**
 * Свёрнутый сайдбар не оставляет после себя ничего: ни рельса иконок,
 * ни полосы — контент занимает весь экран. Достать меню можно двумя
 * способами: кнопкой в шапке контента и наведением на левый край,
 * от которого сайдбар всплывает поверх (peek) и уезжает, как только
 * курсор ушёл.
 */
export function Sidebar() {
  const { sidebarCollapsed } = useUi();
  // Корневой уровень: бэкенд требует parent_id всегда, без него он вернёт
  // не список, а сам корневой пункт.
  const { items, isLoading, error } = useMenuChildren(ROOT_MENU_ID);
  const [peeking, setPeeking] = useState(false);
  const stopPeek = useCallback(() => setPeeking(false), []);

  useEffect(() => {
    if (!sidebarCollapsed) setPeeking(false);
  }, [sidebarCollapsed]);

  if (!sidebarCollapsed) {
    return <ExpandedSidebar menus={items} isLoading={isLoading} error={error} />;
  }

  return (
    <>
      {/* Полоса-ловушка у самого края: попасть в неё можно броском мыши
          влево, не целясь в кнопку. */}
      <div
        className="fixed inset-y-0 left-0 z-30 w-2"
        onPointerEnter={() => setPeeking(true)}
        aria-hidden
      />

      {peeking && (
        <ExpandedSidebar
          menus={items}
          isLoading={isLoading}
          error={error}
          onLeave={stopPeek}
        />
      )}
    </>
  );
}

/** Кнопка в шапке контента: единственный способ вернуть сайдбар насовсем. */
export function SidebarToggleButton() {
  const { t } = useTranslation();
  const { sidebarCollapsed, toggleSidebar } = useUi();

  if (!sidebarCollapsed) return null;

  return (
    <button
      type="button"
      onClick={toggleSidebar}
      aria-label={t("sidebar.open")}
      title={t("sidebar.open")}
      className="grid size-7 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={IconLayoutSidebarLeftExpand} size={16} />
    </button>
  );
}

function ExpandedSidebar({
  menus,
  isLoading,
  error,
  onLeave,
}: {
  menus: MenuNode[];
  isLoading: boolean;
  error: Error | null;
  /** Задан только у всплывающего сайдбара: закрыться, когда курсор ушёл. */
  onLeave?: () => void;
}) {
  const { t } = useTranslation();
  const { sidebarWidth, setSidebarWidth } = useUi();
  const aside = useRef<HTMLElement>(null);

  /*
   * Уход курсора ловится на документе, а не через onPointerLeave:
   * поповер рабочего пространства шире сайдбара и торчит за его края,
   * но лежит внутри него в DOM — contains() это учитывает, геометрия
   * нет, и меню закрывалось бы прямо под курсором.
   */
  useEffect(() => {
    if (!onLeave) return;

    const onOver = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) return;

      /*
       * Модальное окно, открытое отсюда, лежит порталом в <body>: курсор,
       * доехавший до него, формально уходит из панели. Без этой проверки
       * панель закрывалась бы по дороге к кнопке и уносила окно с собой —
       * оно живёт в её поддереве. Та же причина, что и в Popover.
       */
      if (target.closest("[data-modal]")) return;

      if (!aside.current?.contains(target)) onLeave();
    };

    document.addEventListener("pointerover", onOver);
    return () => document.removeEventListener("pointerover", onOver);
  }, [onLeave]);

  return (
    <aside
      ref={aside}
      style={{ width: sidebarWidth }}
      className={
        onLeave
          /*
           * Выехавший сайдбар — во всю высоту и вплотную к краю, как
           * и закреплённый: это тот же сайдбар, показанный на время,
           * а не новый плавающий объект. Отступ сверху и снизу был
           * третьим ритмом на экране — ни 0 у контента, ни 8px у карточки.
           * Что он временный, говорят тень и выезд, а не зазор.
           *
           * z-55: выше карточки записи (z-50). Широкая карточка накрывает
           * левый край экрана целиком, и панель, вызванная наведением,
           * уезжала под неё — то есть не появлялась вовсе.
           */
          ? "group/aside animate-peek fixed inset-y-0 left-0 z-55 flex flex-col gap-2 rounded-r-xl border-r border-border bg-surface p-2 shadow-modal"
          : "group/aside relative flex shrink-0 flex-col gap-2 bg-bg p-2"
      }
    >
      {/* У всплывающего ручки нет: тянуть край панели, которая закроется,
          стоит курсору выйти за него, — занятие на любителя. */}
      {!onLeave && (
        <ResizeHandle
          edge="right"
          target={aside}
          value={sidebarWidth}
          min={SIDEBAR_MIN_WIDTH}
          max={SIDEBAR_MAX_WIDTH}
          label={t("sidebar.resize")}
          onCommit={setSidebarWidth}
        />
      )}

      <WorkspaceHeader floating={Boolean(onLeave)} />

      <div className="flex items-center justify-between px-2 pt-1">
        <span className="text-2xs font-medium tracking-wide text-fg-subtle uppercase">
          {t("sidebar.menu")}
        </span>
        <AddMenuButton parentId={ROOT_MENU_ID} />
      </div>

      <nav className="flex-1 overflow-y-auto" aria-label={t("sidebar.menu")}>
        {isLoading && <Skeleton />}
        {error && <LoadError error={error} />}
        {!isLoading && !error && menus.length === 0 && (
          <p className="px-2 py-1 text-xs text-fg-subtle">{t("sidebar.empty")}</p>
        )}
        <MenuDndProvider>
          <MenuLevel items={menus} path={[ROOT_MENU_ID]} />
        </MenuDndProvider>
      </nav>
    </aside>
  );
}

/**
 * Ошибка показывается словами сервера, а не общей фразой: «project id is
 * an invalid uuid» чинится за минуту, «не удалось загрузить» — за час.
 */
function LoadError({ error }: { error: Error }) {
  const { t } = useTranslation();
  // Разбирает ответ общий errorText: причина приходит и голой строкой,
  // и завёрнутой в {message} — своя проверка знала только первую форму
  // и на самом частом ответе показывала пустоту.
  const detail = errorText(error);

  return (
    <div className="mx-1 flex flex-col gap-1 rounded-md bg-danger-subtle px-2 py-1.5">
      <p className="text-xs font-medium text-danger">{t("sidebar.loadFailed")}</p>
      {detail && <p className="text-2xs break-words text-danger opacity-80">{detail}</p>}
    </div>
  );
}

/** Скелетон повторяет высоту строки — сайдбар не «прыгает» после загрузки. */
function Skeleton() {
  return (
    <div className="flex flex-col gap-0.5" aria-hidden>
      {[70, 55, 80, 60, 45].map((width, index) => (
        <div key={index} className="flex h-8 items-center px-2">
          <div className="h-3 rounded-sm bg-surface-active" style={{ width: `${width}%` }} />
        </div>
      ))}
    </div>
  );
}
