import type { QueryClient } from "@tanstack/react-query";
import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import { FilePreview } from "@/shared/ui/file-preview";
import { Toaster } from "@/shared/ui/toaster";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    /* Фон не свой, а от body: там градиент, и вторая заливка поверх
       превратила бы его обратно в плоский цвет. */
    <div className="min-h-dvh text-fg">
      <Outlet />
      {/* Один на всё приложение: уведомление приходит из мутации,
          а не из экрана, и переживает переход между маршрутами. */}
      <Toaster />
      {/* Просмотр файла — тоже один: его открывают из ячейки, карточки
          и редактора поля, а окно должно быть одно и то же. */}
      <FilePreview />
    </div>
  ),
});
