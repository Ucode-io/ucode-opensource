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
  /**
   * Правым краем к кнопке — когда справа не помещается: у чипов
   * в панели настроек view это норма, панель и так стоит у края экрана.
   */
  const [end, setEnd] = useState(false);
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
      setEnd(false);
      return;
    }

    const trigger = root.current?.getBoundingClientRect();
    const box = menu.current?.getBoundingClientRect();
    if (!trigger || !box) return;

    // Переворачиваем, только если сверху места действительно больше.
    setUp(trigger.bottom + box.height > window.innerHeight && trigger.top > box.height);

    /*
     * То же вбок — но по краю того, кто меню обрежет, а не по краю окна:
     * контент лежит в карточке с отступом, и меню, влезающее в экран,
     * всё равно оказывалось бы срезанным её краем.
     */
    const bounds = clipBounds(menu.current);
    setEnd(trigger.left + box.width > bounds.right && trigger.right - box.width > bounds.left);

    /*
     * Меню лежит внутри своего родителя, и если тот прокручивается
     * (список полей в панели настроек, список пунктов в сайдбаре), край
     * прокрутки его обрезает: видно половину строки, и добраться до
     * остальных нечем. Разворот вверх тут не спасает — он считается по
     * окну, а режет контейнер.
     *
     * `nearest` подкручивает ровно тот контейнер, который мешает, и
     * ничего не делает, когда меню и так видно целиком.
     *
     * Но только по вертикали. Горизонтальную прокрутку браузер применяет
     * к карточке контента — у неё `overflow-hidden`, а такой контейнер
     * scrollIntoView всё равно прокручивает: меню у правого края уводило
     * вбок весь экран вместе с таблицей, и вернуть его было нечем —
     * полосы прокрутки у скрытого переполнения нет. Запоминаем смещения
     * предков и возвращаем на место; всё это до кадра, поэтому не мигает.
     */
    const scrolled: [Element, number][] = [];
    for (let node = menu.current?.parentElement; node; node = node.parentElement) {
      // Нули тоже: именно из нуля контейнер и уезжает.
      scrolled.push([node, node.scrollLeft]);
    }

    menu.current?.scrollIntoView({ block: "nearest", inline: "nearest" });

    for (const [node, left] of scrolled) node.scrollLeft = left;
  }, [open]);

  useEffect(() => {
    if (!open) return;

    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) return;

      /*
       * Модальное окно, открытое из меню, лежит порталом в <body> — то
       * есть DOM-снаружи, хотя по смыслу оно внутри. Без этой проверки
       * нажатие на его кнопку читалось как «мимо»: меню закрывалось,
       * уносило с собой окно (оно живёт в поддереве пункта меню), и click
       * до кнопки уже не долетал. «Выйти из аккаунта?» исчезало, ничего
       * не сделав.
       */
      if (target.closest("[data-modal]")) return;

      if (!root.current?.contains(target)) setOpen(false);
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
            align === "end" || end ? "right-0" : "left-0"
          } ${up ? "bottom-full mb-1" : "top-full mt-1"}`}
        >
          {children(() => setOpen(false))}
        </div>
      )}
    </div>
  );
}

/**
 * Границы, за которые меню не пустят: самый узкий из обрезающих предков,
 * а если таких нет — окно.
 *
 * Обрезает не только `overflow: hidden`: `clip`, `auto` и `scroll` — тоже.
 * Поэтому проверяется «не visible», а не конкретное значение.
 */
function clipBounds(element: HTMLElement | null): { left: number; right: number } {
  let left = 0;
  let right = window.innerWidth;

  for (let node = element?.parentElement; node; node = node.parentElement) {
    if (getComputedStyle(node).overflowX === "visible") continue;

    const rect = node.getBoundingClientRect();
    left = Math.max(left, rect.left);
    right = Math.min(right, rect.right);
  }

  return { left, right };
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
