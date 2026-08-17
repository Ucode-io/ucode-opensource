import { create } from "zustand";
import { persist } from "zustand/middleware";

type Theme = "light" | "dark" | "system";

type UiState = {
  theme: Theme;
  sidebarCollapsed: boolean;
  sidebarWidth: number;
  drawerWidth: number;
  drawerMode: DrawerMode;
  /**
   * Последний выбранный размер страницы — по слагу таблицы.
   *
   * Сама пагинация живёт в адресе, но человек выбирает размер страницы
   * один раз и ждёт его во всех таблицах, куда вернётся: без памяти
   * каждый переход туда-обратно возвращал бы значение по умолчанию.
   */
  tableLimits: Record<string, number>;
  /**
   * Последний отбор — по паре «таблица + view».
   *
   * Значения фильтров живут в адресе, чтобы ссылку можно было переслать,
   * но человек, вернувшийся на экран без параметров, ждёт свой отбор
   * на месте — так было и в старой админке (слайс filter в localStorage).
   * Форму значения знает features/item, здесь оно непрозрачно и
   * проверяется при чтении.
   */
  tableFilters: Record<string, unknown>;
  /**
   * Раскрытые папки меню — по id пункта.
   *
   * Раскрытие живёт здесь, а не в строке меню: уровень меню грузится
   * своим запросом, и без памяти каждый рефреш схлопывал бы дерево
   * до корня — вместе с путём к тому пункту, в котором человек работает.
   *
   * Список, а не карта: закрытая папка из него уходит, и вырасти он
   * может только до числа одновременно раскрытых. ponytail: id
   * удалённого пункта останется в списке навсегда, но стоит он строку
   * и ни на что не влияет.
   */
  expandedMenus: string[];
  /**
   * Язык ДАННЫХ, на котором показаны значения и подписи. Не локаль
   * интерфейса: язык интерфейса выбирает человек для себя, а этот —
   * ось самих данных (см. CONTEXT, Data Language).
   *
   * Пусто — язык не выбирали: берётся первый язык проекта. Хранить
   * приходится, потому что переключатель стоит и в карточке, и над
   * таблицей, и выбор обязан пережить переход между ними.
   */
  dataLanguage: string;
  setTheme: (theme: Theme) => void;
  toggleSidebar: () => void;
  toggleMenu: (id: string) => void;
  setDataLanguage: (code: string) => void;
  setSidebarWidth: (width: number) => void;
  setDrawerWidth: (width: number) => void;
  setDrawerMode: (mode: DrawerMode) => void;
  setTableLimit: (tableSlug: string, limit: number) => void;
  setTableFilters: (key: string, filters: unknown) => void;
};

/**
 * Три положения drawer'а: сбоку, по центру и во весь экран. Это одна
 * и та же панель в разных местах экрана, а не три компонента.
 */
export type DrawerMode = "side" | "center" | "full";

/**
 * Ширина сайдбара ограничена здесь, а не в компоненте: тянуть мышью и
 * жать стрелки — два пути к одному значению, клампить надо один раз.
 * Ниже 200px пункты меню обрезаются, выше 480 сайдбар отъедает таблицу.
 */
export const SIDEBAR_MIN_WIDTH = 200;
export const SIDEBAR_MAX_WIDTH = 480;
export const SIDEBAR_DEFAULT_WIDTH = 256;

export const DRAWER_MIN_WIDTH = 400;
export const DRAWER_MAX_WIDTH = 1200;
export const DRAWER_DEFAULT_WIDTH = 520;

export const clampSidebarWidth = (width: number) =>
  clamp(width, SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH);

export const clampDrawerWidth = (width: number) => clamp(width, DRAWER_MIN_WIDTH, DRAWER_MAX_WIDTH);

const clamp = (value: number, min: number, max: number) =>
  Math.round(Math.min(max, Math.max(min, value)));

/**
 * Единственный глобальный стор. Всё остальное — в URL (фильтры, view,
 * пагинация) или в TanStack Query (данные). В старом ucode в localStorage
 * персистилось ~30 redux-слайсов и redux-persist писал их синхронно
 * на каждый dispatch.
 */
export const useUi = create<UiState>()(
  persist(
    (set) => ({
      theme: "system",
      sidebarCollapsed: false,
      sidebarWidth: SIDEBAR_DEFAULT_WIDTH,
      drawerWidth: DRAWER_DEFAULT_WIDTH,
      drawerMode: "side",
      tableLimits: {},
      tableFilters: {},
      expandedMenus: [],
      dataLanguage: "",
      setTheme: (theme) => set({ theme }),
      setDataLanguage: (dataLanguage) => set({ dataLanguage }),
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      toggleMenu: (id) =>
        set((s) => ({
          expandedMenus: s.expandedMenus.includes(id)
            ? s.expandedMenus.filter((item) => item !== id)
            : [...s.expandedMenus, id],
        })),
      setSidebarWidth: (width) => set({ sidebarWidth: clampSidebarWidth(width) }),
      setDrawerWidth: (width) => set({ drawerWidth: clampDrawerWidth(width) }),
      setDrawerMode: (drawerMode) => set({ drawerMode }),
      setTableLimit: (tableSlug, limit) =>
        set((s) => ({ tableLimits: { ...s.tableLimits, [tableSlug]: limit } })),
      setTableFilters: (key, filters) =>
        set((s) => ({ tableFilters: { ...s.tableFilters, [key]: filters } })),
    }),
    { name: "ucode.ui" },
  ),
);

export function applyTheme(theme: Theme) {
  const dark =
    theme === "dark" ||
    (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
}
