import { Outlet, createFileRoute, redirect } from "@tanstack/react-router";
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

  return (
    // overflow-hidden здесь нельзя: кнопка сворачивания сайдбара торчит
    // за его границу и была бы обрезана. Прокрутку держат сами колонки.
    <div className="flex h-dvh">
      <Sidebar />
      {/*
        Контент — карточка на фоне приложения: отступ со всех сторон,
        своя граница и скругление. 12px — самый крупный радиус в системе,
        и он здесь по правилу «чем крупнее поверхность, тем больше радиус»
        (docs/DESIGN.md). Граница на карточке, а не на сайдбаре: с зазором
        она читается как край панели, а не как разделитель во всю высоту.
      */}
      {/* Без сайдбара карточки нет: отступ и скругление слева не от чего
          отделять, и контент занимает экран целиком. */}
      <main
        className={`flex min-w-0 flex-1 flex-col overflow-hidden bg-surface ${
          sidebarCollapsed ? "" : "m-2 rounded-xl border border-border"
        }`}
      >
        <Outlet />
      </main>
    </div>
  );
}
