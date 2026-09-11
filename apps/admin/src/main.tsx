import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { GoogleOAuthProvider } from "@react-oauth/google";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { installRefreshHandler } from "./features/auth";
import { clearLegacyMirror } from "./features/microfrontend";
import { session } from "./shared/api/session";
import { queryClient } from "./app/query-client";
import { applyTheme, useUi } from "./shared/lib/ui-store";
import { googleClientId, isGoogleLoginEnabled } from "./shared/lib/google-oauth";
import { routeTree } from "./routeTree.gen";
import "./shared/lib/i18n";
import "./app/styles.css";

// Клиент узнаёт, как обновлять токен, ровно один раз при старте.
installRefreshHandler();

/*
 * Зеркало сессии для микрофронтендов живёт только пока открыт их экран,
 * но вкладку закрывают и посреди работы. Стираем хвост на старте.
 *
 * Заодно уносит и хвост СТАРОЙ админки: ключи те же, а её дев-сервер
 * слушает тот же порт 7777. Именно оттуда ремоут однажды подобрал токен
 * восьмидневной давности — см. docs/adr/0005.
 */
clearLegacyMirror();

applyTheme(useUi.getState().theme);
useUi.subscribe((state) => applyTheme(state.theme));

const router = createRouter({ routeTree, context: { queryClient } });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

/**
 * Refresh-токен тоже истекает. Когда обновить сессию больше нечем,
 * auth её очищает — и здесь мы уводим на вход. Без этого человек
 * остался бы на странице, где каждый запрос молча возвращает ошибку.
 */
session.subscribe(() => {
  if (session.isAuthenticated()) return;
  if (router.state.location.pathname.startsWith("/login")) return;
  void router.navigate({ to: "/login" });
});

const app = (
  <QueryClientProvider client={queryClient}>
    <RouterProvider router={router} />
  </QueryClientProvider>
);

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {isGoogleLoginEnabled ? (
      <GoogleOAuthProvider clientId={googleClientId}>{app}</GoogleOAuthProvider>
    ) : (
      app
    )}
  </StrictMode>,
);
