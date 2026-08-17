import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from "react";

/**
 * Всплывающее меню. Своё, а не библиотека: нужно закрытие по клику мимо,
 * по Escape и позиционирование под кнопкой — это три обработчика.
 */
export function Popover({
  trigger,
  children,
  align = "start",
}: {
  trigger: (props: { open: boolean; toggle: () => void }) => ReactNode;
  children: (close: () => void) => ReactNode;
  align?: "start" | "end";
}) {
  const [open, setOpen] = useState(false);
  /** Вверх — когда снизу не помещается: у нижних пунктов сайдбара это норма. */
  const [up, setUp] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const menu = useRef<HTMLDivElement>(null);

  /**
   * Направление считается по факту, после отрисовки: высота зависит от числа
   * пунктов, а место под кнопкой — от прокрутки. useLayoutEffect, а не
   * useEffect: замер и переворот должны успеть до кадра, иначе меню мигает.
   */
  useLayoutEffect(() => {
    if (!open) {
      setUp(false);
      return;
    }

    const trigger = root.current?.getBoundingClientRect();
    const box = menu.current?.getBoundingClientRect();
    if (!trigger || !box) return;

    // Переворачиваем, только если сверху места действительно больше.
    setUp(trigger.bottom + box.height > window.innerHeight && trigger.top > box.height);

    /*
     * Меню лежит внутри своего родителя, и если тот прокручивается
     * (список полей в панели настроек, список пунктов в сайдбаре), край
     * прокрутки его обрезает: видно половину строки, и добраться до
     * остальных нечем. Разворот вверх тут не спасает — он считается по
     * окну, а режет контейнер.
     *
     * `nearest` подкручивает ровно тот контейнер, который мешает, и
     * ничего не делает, когда меню и так видно целиком.
     */
    menu.current?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [open]);

  useEffect(() => {
    if (!open) return;

    const onPointerDown = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div ref={root} className="relative">
      {trigger({ open, toggle: () => setOpen((v) => !v) })}

      {open && (
        <div
          ref={menu}
          role="menu"
          className={`absolute z-50 min-w-48 rounded-lg border border-border bg-surface p-1 shadow-popover ${
            align === "end" ? "right-0" : "left-0"
          } ${up ? "bottom-full mb-1" : "top-full mt-1"}`}
        >
          {children(() => setOpen(false))}
        </div>
      )}
    </div>
  );
}

export function PopoverItem({
  icon,
  children,
  onClick,
  danger,
  active,
  trailing,
}: {
  icon?: ReactNode;
  children: ReactNode;
  /** Событие нужно тем пунктам, что открывают панель под собой: им нужен якорь. */
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
  danger?: boolean;
  /** Переключатель включён — пункт красится фирменным цветом. */
  active?: boolean;
  /** Значение справа: счётчик, стрелка вложенной страницы. */
  trailing?: ReactNode;
}) {
  const tone = danger
    ? "text-danger hover:bg-danger-subtle"
    : active
      ? "bg-accent-subtle text-accent-text"
      : "text-fg hover:bg-surface-hover";

  return (
    <button
      type="button"
      role="menuitem"
      aria-pressed={active}
      onClick={onClick}
      className={`flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors ${tone}`}
    >
      {icon}
      <span className="flex-1 truncate">{children}</span>
      {trailing}
    </button>
  );
}

export function PopoverSeparator() {
  return <div className="my-1 h-px bg-border" />;
}
