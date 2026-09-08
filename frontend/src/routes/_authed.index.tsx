import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AddMenuButton, ROOT_MENU_ID, useMenuChildren } from "@/features/sidebar";

export const Route = createFileRoute("/_authed/")({ component: Home });

/**
 * Экран без выбранного пункта меню.
 *
 * Две разные пустоты, а не одна фраза на оба случая: в проекте с меню
 * человек просто ещё ничего не открыл, и ему нечего создавать — нужно
 * выбрать. В пустом проекте выбирать не из чего, и единственное
 * осмысленное действие здесь — завести первый пункт.
 *
 * Своего запроса нет: список верхнего уровня уже везёт сайдбар, и
 * берётся тот же ответ из кэша.
 */
function Home() {
  const { t } = useTranslation();
  const { items, isLoading } = useMenuChildren(ROOT_MENU_ID);

  // Пока список едет — только картинка: подпись «создайте первый пункт»,
  // мигнувшая перед списком из двадцати, врёт.
  const empty = !isLoading && items.length === 0;

  return (
    <div className="animate-page grid h-full place-items-center p-8">
      <div className="flex flex-col items-center">
        <CardStack />

        {!isLoading && (
          <p className="mt-10 text-sm text-fg-muted">
            {t(empty ? "home.empty" : "home.pick")}
          </p>
        )}

        {empty && (
          <div className="mt-4">
            <AddMenuButton parentId={ROOT_MENU_ID} label={t("sidebar.add")} />
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Стопка ненастоящих карточек: то, что появится здесь, когда пункт
 * выберут. Это рисунок, а не скелетон загрузки — поэтому без пульсации:
 * мигающий заполнитель обещает, что сейчас что-то придёт, а здесь
 * ничего не едет.
 *
 * Три div'а вместо картинки: SVG пришлось бы рисовать дважды — под
 * светлую тему и под тёмную, — а здесь цвета берутся токенами.
 */
function CardStack() {
  return (
    <div aria-hidden className="relative h-40 w-72">
      <GhostCard className="top-0 left-14 -rotate-6 opacity-40" />
      <GhostCard className="top-4 left-2 rotate-6 opacity-60" />
      <GhostCard className="top-8 left-8" />
    </div>
  );
}

function GhostCard({ className }: { className: string }) {
  return (
    <div
      className={`absolute h-28 w-48 rounded-xl border border-border bg-surface p-3 shadow-card ${className}`}
    >
      <div className="flex gap-2">
        <div className="size-9 shrink-0 rounded-md bg-surface-active" />
        <div className="flex flex-1 flex-col gap-1.5 pt-1">
          <div className="h-2 rounded-full bg-surface-active" />
          <div className="h-2 w-2/3 rounded-full bg-surface-active" />
        </div>
      </div>
      <div className="mt-3 flex flex-col gap-1.5">
        <div className="h-2 rounded-full bg-surface-active" />
        <div className="h-2 w-4/5 rounded-full bg-surface-active" />
      </div>
    </div>
  );
}
