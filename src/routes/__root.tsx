import type { QueryClient } from "@tanstack/react-query";
import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import { Toaster } from "@/shared/ui/toaster";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <div className="min-h-dvh bg-bg text-fg">
      <Outlet />
      {/* Один на всё приложение: уведомление приходит из мутации,
          а не из экрана, и переживает переход между маршрутами. */}
      <Toaster />
    </div>
  ),
});
