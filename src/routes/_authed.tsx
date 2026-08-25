import { useEffect } from "react";
import { Outlet, createFileRoute, redirect } from "@tanstack/react-router";
import { CopilotPanel } from "@/features/copilot";
import { useProject } from "@/features/settings";
import { Sidebar } from "@/features/sidebar";
import { useUi } from "@/shared/lib/ui-store";
import { ensureAccessToken } from "@/shared/api/client";
import { session } from "@/shared/api/session";

/**
 * Всё под этим маршрутом требует сессии. Проверка одна и в одном месте —
 * в старом ucode guard'ы были размазаны по компонентам вперемешку
 * с проверками legacy-флагов.
 *
 * Здесь же восстанавливается access-токен: он живёт только в памяти, и
 * после перезагрузки страницы его нужно получить до первого запроса.
 * Иначе запрос уходит без Authorization, а шлюз отвечает на это 403.
 */
export const Route = createFileRoute("/_authed")({
  beforeLoad: async ({ location }) => {
    const toLogin = redirect({ to: "/login", search: { redirect: location.href } });

    if (!session.isAuthenticated()) throw toLogin;
    if (!(await ensureAccessToken())) {
      session.clear();
      throw toLogin;
    }
  },
  component: AppShell,
});

function AppShell() {
  const { sidebarCollapsed } = useUi();
  useProjectBranding();

  return (
    // overflow-hidden здесь нельзя: кнопка сворачивания сайдбара торчит
    // за его границу и была бы обрезана. Прокрутку держат сами колонки.
    <div className="flex h-dvh">
      <Sidebar />
      {/*
        Контент — карточка на фоне приложения: отступ со всех сторон,
        скругление и мягкая тень. 12px — самый крупный радиус в системе,
        и он здесь по правилу «чем крупнее поверхность, тем больше радиус»
        (docs/DESIGN.md). Край держит тень, а не граница: фон под карточкой
        теперь с градиентом, и волосяная линия на нём читалась как шов.
      */}
      {/* Без сайдбара карточки нет: контент занимает экран целиком, и ни
          скругление, ни тень не к чему прислонить — тень по краю экрана
          не видна, зато обрезается вместе с ним. */}
      {/*
        overflow-clip, а не hidden. Разница не косметическая: `hidden`
        заводит контейнер прокрутки — без полосы, но прокручиваемый
        программно. Всплывающее меню, вылезшее за правый край карточки,
        браузер «показывал» именно так: уводил всю карточку вбок вместе
        с таблицей, а вернуть её было нечем. `clip` просто обрезает.
      */}
      <main
        className={`flex min-w-0 flex-1 flex-col overflow-clip bg-surface ${
          sidebarCollapsed ? "" : "m-2 rounded-xl shadow-card"
        }`}
      >
        <Outlet />
      </main>

      {/* Помощник отодвигает контент, а не накрывает его: он правит то,
          что на экране, и таблица должна остаться видна. Смонтирован
          всегда — закрытая панель не теряет начатый разговор. */}
      <CopilotPanel />
    </div>
  );
}

/**
 * Вкладка браузера — под проект: его имя в заголовке, его логотип
 * вместо значка. У человека открыто пять вкладок ucode с разными
 * проектами, и различить их иначе нечем — адрес у всех одинаковый.
 *
 * Так же это делает старая админка (MainLayout: `document.title` и
 * `<Favicon url={projectInfo.logo}/>`). Пакета ради значка здесь нет:
 * react-favicon — это те же четыре строки с DOM, только чужие.
 *
 * Запроса тоже нет: карточку проекта уже грузит features/workspace,
 * и заголовок с логотипом берутся из того же ответа, что языки данных
 * и шапка сайдбара.
 *
 * Оба значения возвращаются на место при выходе из приложения: экран
 * входа — общий для всех проектов, и чужой логотип на нём врёт.
 */
function useProjectBranding() {
  const { project } = useProject();
  const title = project?.title ?? "";
  const logo = project?.logo ?? "";

  useEffect(() => {
    if (!title) return;

    document.title = title;
    return () => {
      document.title = DEFAULT_TITLE;
    };
  }, [title]);

  useEffect(() => {
    if (!logo) return;

    /*
     * <link rel="icon"> в index.html нет — значок берётся по умолчанию
     * из /favicon.ico. Заводим свой, когда есть что в него положить,
     * и возвращаем прежний адрес при уходе: пустой href — это и есть
     * «как было», браузер снова спросит /favicon.ico.
     */
    let link = document.querySelector<HTMLLinkElement>("link[rel='icon']");

    if (!link) {
      link = document.createElement("link");
      link.rel = "icon";
      document.head.append(link);
    }

    const before = link.getAttribute("href") ?? "";
    link.href = logo;

    return () => {
      link.setAttribute("href", before);
    };
  }, [logo]);
}

/** Заголовок вкладки вне проекта — тот же, что в index.html. */
const DEFAULT_TITLE = "ucode";
