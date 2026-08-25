import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { IconChevronsRight, IconSearch } from "@tabler/icons-react";
import { Link } from "@tanstack/react-router";
import { errorText } from "@/shared/api/client";
import { Icon } from "@/shared/ui/icon";
import { ResizeHandle } from "@/shared/ui/resize-handle";
import { SIDEBAR_MAX_WIDTH, SIDEBAR_MIN_WIDTH, useUi } from "@/shared/lib/ui-store";
import { ROOT_MENU_ID, useMenuChildren, useMenuTree } from "../api/menus";
import { matchMenus, type MenuMatch } from "../model/search";
import type { MenuNode } from "../model/types";
import { AddMenuButton } from "./AddMenuButton";
import { MenuDndProvider } from "./dnd-context";
import { MenuIcon } from "./MenuIcon";
import { MenuLevel } from "./MenuLevel";
import { WorkspaceHeader } from "./WorkspaceHeader";

/**
 * Свёрнутый сайдбар не оставляет после себя ничего: ни рельса иконок,
 * ни полосы — контент занимает весь экран. Достать меню можно двумя
 * способами: кнопкой в шапке контента и наведением на левый край,
 * от которого сайдбар всплывает поверх (peek) и уезжает, как только
 * курсор ушёл.
 *
 * Панель ОДНА на все три состояния и с экрана не снимается: свёрнутая
 * уезжает за левый край и ждёт там. Двух копий быть не должно (у них
 * разошлась бы строка поиска), а снять и вернуть — значит показывать
 * рывком: у снятого с экрана нечему ехать обратно.
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

  return (
    <>
      {/* Полоса-ловушка у самого края: попасть в неё можно броском мыши
          влево, не целясь в кнопку. */}
      {sidebarCollapsed && (
        <div
          className="fixed inset-y-0 left-0 z-30 w-2"
          onPointerEnter={() => setPeeking(true)}
          aria-hidden
        />
      )}

      <SidebarPanel
        menus={items}
        isLoading={isLoading}
        error={error}
        collapsed={sidebarCollapsed}
        peeking={sidebarCollapsed && peeking}
        onLeave={stopPeek}
      />
    </>
  );
}

/** Кнопка в шапке контента: единственный способ вернуть сайдбар насовсем. */
export function SidebarToggleButton() {
  const { t } = useTranslation();
  const { sidebarCollapsed, toggleSidebar } = useUi();

  return (
    <button
      type="button"
      onClick={toggleSidebar}
      aria-label={t("sidebar.open")}
      title={t("sidebar.open")}
      /* Закреплённому сайдбару кнопка не нужна, но снимать её с места
         нельзя: заголовок страницы прыгнул бы влево ровно в тот момент,
         когда сайдбар плавно выезжает. Поэтому она схлопывается, а
         отрицательный отступ съедает зазор ряда (gap-2), который у
         нулевой ширины остался бы лишним. */
      inert={!sidebarCollapsed}
      className={`grid h-7 shrink-0 place-items-center overflow-hidden rounded-md text-fg-muted transition-[width,margin,background-color,color] duration-200 ease-out hover:bg-surface-hover hover:text-fg ${
        sidebarCollapsed ? "w-7" : "-mr-2 w-0"
      }`}
    >
      {/* Стрелки вправо — зеркало «свернуть» в шапке сайдбара
          (WorkspaceHeader, CollapseButton): направление, а не картинка
          панели, которой сейчас нет на экране. */}
      <Icon as={IconChevronsRight} size={16} />
    </button>
  );
}

function SidebarPanel({
  menus,
  isLoading,
  error,
  collapsed,
  peeking,
  onLeave,
}: {
  menus: MenuNode[];
  isLoading: boolean;
  error: Error | null;
  /** Убран с экрана: уехал за левый край, места в раскладке не занимает. */
  collapsed: boolean;
  /** Свёрнутый, но вызванный наведением: лежит ПОВЕРХ контента. */
  peeking: boolean;
  /** Закрыться, когда курсор ушёл. Слушается только у всплывающего. */
  onLeave: () => void;
}) {
  const { t } = useTranslation();
  const { sidebarWidth, setSidebarWidth } = useUi();
  const aside = useRef<HTMLElement>(null);
  /*
   * Поиск идёт по ВСЕМУ дереву, а не по загруженным уровням: пункт,
   * который ищут, обычно лежит в неоткрытой папке — иначе его было бы
   * видно и так. Дерево ради этого дочитывается целиком, но только
   * когда в строке что-то есть (см. useMenuTree).
   */
  const [query, setQuery] = useState("");
  const searching = query.trim().length > 0;

  /*
   * Уход курсора ловится на документе, а не через onPointerLeave:
   * поповер рабочего пространства шире сайдбара и торчит за его края,
   * но лежит внутри него в DOM — contains() это учитывает, геометрия
   * нет, и меню закрывалось бы прямо под курсором.
   */
  useEffect(() => {
    if (!peeking) return;

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
  }, [peeking, onLeave]);

  return (
    <aside
      ref={aside}
      /*
       * Свёрнутый уезжает ОТРИЦАТЕЛЬНЫМ ОТСТУПОМ, а не нулевой шириной:
       * ширину в это же время пишет ручка размера (см. ResizeHandle),
       * и переход по ней превратил бы перетаскивание в желе. Отступ
       * же двигает и панель, и контент за ней — одним свойством.
       *
       * Всплывающий сдвигается поверх контента сдвигом (`translate`):
       * раскладку он не трогает, поэтому таблица под ним не дёргается.
       * Сумма даёт ровно край экрана: -width + 100% = 0.
       */
      style={{ width: sidebarWidth, marginLeft: collapsed ? -sidebarWidth : 0 }}
      /* Уехавшая панель остаётся в DOM ради обратного хода — но не в
         порядке обхода с клавиатуры: Tab не должен уводить в невидимое. */
      inert={collapsed && !peeking}
      /* Переход по `translate`, а не по `transform`: утилиты сдвига
         в tailwind 4 пишут отдельное свойство translate. */
      className={`group/aside relative flex shrink-0 flex-col gap-2 p-2 transition-[margin-left,translate,background-color,box-shadow] duration-200 ease-out ${
        collapsed
          /*
           * Свёрнутый — плавающая поверхность: заливка, тень и скруглённый
           * правый край. Не только ради вида наведением вызванной панели:
           * пока она уезжает и приезжает, под ней едет контент, и панель
           * без своей заливки просвечивала бы насквозь.
           *
           * z-55: выше карточки записи (z-50). Широкая карточка накрывает
           * левый край экрана целиком, и панель, вызванная наведением,
           * уезжала под неё — то есть не появлялась вовсе.
           */
          ? `z-55 rounded-r-xl border-r border-border bg-surface shadow-modal ${peeking ? "translate-x-full" : ""}`
          /* Закреплённый — без своей заливки: он лежит на фоне приложения
             и берёт его градиент, а не гасит его плоским bg. Контент
             (`main`, z-10) при этом лежит ВЫШЕ, и раскрытие читается как
             «контент отъехал и открыл меню», а не как наложение. */
          : ""
      }`}
    >
      {/* У свёрнутого ручки нет: тянуть край панели, которая закроется,
          стоит курсору выйти за него, — занятие на любителя. */}
      {!collapsed && (
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

      <WorkspaceHeader floating={peeking} />

      {/* Поле поиска говорит на языке панели, а не формы: та же высота 32,
          тот же радиус и та же пара «заливка при наведении → surface плюс
          тень в фокусе», что у строк меню. Рамка формы была здесь
          единственной, и поле читалось как выбранный пункт.

          type="search" — ради встроенного крестика: очистка строки уже
          есть в браузере, своя кнопка была бы второй такой же. */}
      <label className="flex h-8 items-center gap-2 rounded-md bg-surface/60 px-2 text-sm transition-colors focus-within:bg-surface focus-within:shadow-raised">
        <Icon as={IconSearch} size={16} className="text-fg-subtle" />
        <input
          type="search"
          value={query}
          placeholder={t("sidebar.search")}
          aria-label={t("sidebar.search")}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => event.key === "Escape" && setQuery("")}
          className="min-w-0 flex-1 bg-transparent text-fg outline-none placeholder:text-fg-subtle"
        />
      </label>

      <div className="flex items-center justify-between px-2 pt-1">
        <span className="text-2xs font-medium tracking-wide text-fg-subtle uppercase">
          {t(searching ? "sidebar.searchResults" : "sidebar.menu")}
        </span>
        <AddMenuButton parentId={ROOT_MENU_ID} />
      </div>

      <nav className="flex-1 overflow-y-auto" aria-label={t("sidebar.menu")}>
        {searching ? (
          <SearchResults query={query} onPicked={() => setQuery("")} />
        ) : (
          <>
            {isLoading && <Skeleton />}
            {error && <LoadError error={error} />}
            {!isLoading && !error && menus.length === 0 && (
              <p className="px-2 py-1 text-xs text-fg-subtle">{t("sidebar.empty")}</p>
            )}
            <MenuDndProvider>
              <MenuLevel items={menus} path={[ROOT_MENU_ID]} />
            </MenuDndProvider>
          </>
        )}
      </nav>
    </aside>
  );
}

/**
 * Найденное — плоским списком, а не подсвеченным деревом: в дереве
 * совпадение всё равно пришлось бы показывать вместе с родителями,
 * то есть тем же списком, только с отступами.
 *
 * Под именем — дорога до пункта: две «Заявки» из разных папок иначе
 * неразличимы.
 */
function SearchResults({ query, onPicked }: { query: string; onPicked: () => void }) {
  const { t } = useTranslation();
  const { items, isLoading, error } = useMenuTree(true);
  const matches = useMemo(() => matchMenus(items, query), [items, query]);

  if (isLoading) return <Skeleton />;
  if (error) return <LoadError error={error} />;

  if (matches.length === 0) {
    return <p className="px-2 py-1 text-xs text-fg-subtle">{t("sidebar.searchEmpty")}</p>;
  }

  return (
    <ul className="flex flex-col gap-0.5">
      {matches.map((match) => (
        <li key={match.node.id}>
          <SearchRow match={match} onPicked={onPicked} />
        </li>
      ))}
    </ul>
  );
}

function SearchRow({ match, onPicked }: { match: MenuMatch; onPicked: () => void }) {
  const { node, trail } = match;
  const expandMenus = useUi((state) => state.expandMenus);

  const row =
    "flex h-10 w-full min-w-0 items-center gap-2 rounded-md px-2 text-left text-sm text-fg-muted transition-colors hover:bg-surface-hover";

  const inner = (
    <>
      <span className="grid size-4 shrink-0 place-items-center">
        <MenuIcon name={node.icon} type={node.type} />
      </span>

      <span className="flex min-w-0 flex-col leading-tight">
        <span className="truncate">{node.label}</span>
        {trail.length > 0 && (
          <span className="truncate text-2xs text-fg-subtle">
            {trail.map((step) => step.label).join(" / ")}
          </span>
        )}
      </span>
    </>
  );

  /*
   * Дорога до найденного раскрывается в дереве — и у папки, и у экрана.
   * Иначе поиск, закрывшись, оставляет человека там же, где он был:
   * пункт снова спрятан в неоткрытой папке.
   */
  const reveal = (ids: string[]) => {
    expandMenus(ids);
    onPicked();
  };

  const path = trail.map((step) => step.id);

  // У папки своего экрана нет: щелчок раскрывает её в дереве.
  if (node.kind === "group") {
    return (
      <button type="button" className={row} onClick={() => reveal([...path, node.id])}>
        {inner}
      </button>
    );
  }

  if (node.kind === "link" && node.href) {
    return (
      <a href={node.href} target="_blank" rel="noreferrer" className={row} onClick={onPicked}>
        {inner}
      </a>
    );
  }

  return (
    <Link
      to="/m/$menuId"
      params={{ menuId: node.id }}
      className={row}
      onClick={() => reveal(path)}
    >
      {inner}
    </Link>
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
