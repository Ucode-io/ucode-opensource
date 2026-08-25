import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

/** Отступ меню от кнопки и от края экрана. */
const GAP = 4;
const EDGE = 8;
/** Меню не уже этого, даже если кнопка — квадратный значок. */
const MIN_WIDTH = 192;

/**
 * Открытые меню, от внешнего к внутреннему. Нужен ради Escape: список
 * годов в календаре и календарь — это два меню одно в другом, каждое
 * со своим обработчиком на документе, и без стека одно нажатие
 * закрывало бы оба. Закрывается верхнее.
 */
const opened: Array<() => void> = [];

/**
 * Куда поставить меню. Чистая функция: всё, что зависит от DOM, замерено
 * до неё, — иначе эту арифметику нечем проверить, а именно в ней меню
 * уезжает за край экрана.
 *
 * `bottom` вместо `top` — это разворот вверх: цепляемся нижним краем,
 * и высота меню в расчёт не входит.
 */
export function placement({
  anchor,
  width,
  height,
  view,
  align,
}: {
  anchor: { top: number; bottom: number; left: number; right: number };
  /** Ширина меню — уже с учётом минимальной. */
  width: number;
  /** Высота содержимого, ничем не ограниченная. */
  height: number;
  view: { width: number; height: number };
  align: "start" | "end";
}): { left: number; top?: number; bottom?: number; maxHeight: number } {
  const below = view.height - anchor.bottom - GAP - EDGE;
  const above = anchor.top - GAP - EDGE;
  // Вверх — только если снизу не помещается И сверху места больше.
  const up = height > below && above > below;

  const wanted = align === "end" ? anchor.right - width : anchor.left;

  return {
    left: Math.max(EDGE, Math.min(wanted, view.width - width - EDGE)),
    ...(up ? { bottom: view.height - anchor.top + GAP } : { top: anchor.bottom + GAP }),
    /* Не ниже 96: у кнопки, прижатой к самому низу окна, места нет
       ни там ни там, и без нижней границы меню схлопнулось бы в полоску. */
    maxHeight: Math.max(up ? above : below, 96),
  };
}

/**
 * Всплывающее меню. Своё, а не библиотека: нужно закрытие по клику мимо,
 * по Escape и позиционирование под кнопкой — это три обработчика.
 *
 * Меню уходит ПОРТАЛОМ и позиционируется `fixed` по координатам кнопки.
 * Лежать в потоке оно не может: почти каждая кнопка с меню стоит внутри
 * чего-то прокручиваемого — панель настроек поля, тело модального окна,
 * список колонок, — а прокручиваемый предок обрезает всё, что вылезло
 * за его край, и никакой z-index этого не отменяет. Он же создаёт
 * контекст наложения, из-за которого меню проваливалось под соседа.
 *
 * Портал уходит не всегда в `<body>`: если кнопка живёт внутри модального
 * окна, меню отправляется в это же окно. Так сохраняется лестница слоёв
 * (см. app/styles.css): внутри окна меню лежит над его содержимым, а
 * снаружи — под самим окном. Из `<body>` меню накрывало бы окно, поверх
 * которого его никто не звал.
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
  const root = useRef<HTMLDivElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  /** Куда уходит портал: своё модальное окно или `<body>`. */
  const host = useRef<Element | null>(null);

  if (open) host.current = root.current?.closest("[data-modal]") ?? document.body;

  /**
   * Координаты считаются по факту, после отрисовки: размер зависит от числа
   * пунктов, а место под кнопкой — от прокрутки. useLayoutEffect и правка
   * стиля напрямую, а не через состояние: замер и раскладка должны успеть
   * до кадра, иначе меню мигает не на том месте.
   */
  useLayoutEffect(() => {
    if (!open) return;

    const place = () => {
      const box = menu.current;
      const anchor = root.current?.getBoundingClientRect();
      if (!box || !anchor) return;

      /* Ширину задаём ДО замера: от неё зависит и сам замер, и левый край.
         Меню не уже кнопки — список под полем во всю его ширину. */
      box.style.minWidth = `${Math.max(anchor.width, MIN_WIDTH)}px`;
      box.style.maxWidth = `${window.innerWidth - EDGE * 2}px`;
      // Снимаем прошлую границу: иначе она же и вернётся замером высоты.
      box.style.maxHeight = "";

      const spot = placement({
        anchor,
        width: box.getBoundingClientRect().width,
        height: box.scrollHeight,
        view: { width: window.innerWidth, height: window.innerHeight },
        align,
      });

      box.style.left = `${spot.left}px`;
      box.style.top = spot.top === undefined ? "" : `${spot.top}px`;
      box.style.bottom = spot.bottom === undefined ? "" : `${spot.bottom}px`;
      /* Высота ограничена местом на экране, а не числом пунктов: длинный
         список прокручивается внутри себя, а не уезжает за край. */
      box.style.maxHeight = `${spot.maxHeight}px`;
    };

    place();

    /* Меню приколото к координатам, а кнопка под ним ездит: прокрутка
       любого предка (capture — она не всплывает) и смена размера окна
       пересчитывают положение. */
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [open, align]);

  useEffect(() => {
    if (!open) return;

    const close = () => setOpen(false);
    opened.push(close);

    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) return;

      /*
       * Слой, открытый ИЗ меню, — не «мимо»: ни модальное окно, ни
       * вложенное меню. Без этой проверки нажатие на кнопку такого окна
       * закрывало бы меню, уносило окно с собой (оно живёт в поддереве
       * пункта меню), и click до кнопки уже не долетал: «Выйти
       * из аккаунта?» исчезало, ничего не сделав. С вложенным меню то же
       * самое: выбор года закрывал бы календарь под ним.
       *
       * Сравнение с host обязательно: окно, ВНУТРИ которого живёт само
       * меню, — это обычный фон, клик по нему меню закрывает.
       */
      const layer = target.closest("[data-modal],[data-popover]");
      if (layer && layer !== host.current) return;

      if (root.current?.contains(target)) return;

      close();
    };
    // Закрывается верхнее меню, а не все сразу: см. `opened`.
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") opened.at(-1)?.();
    };

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      opened.splice(opened.indexOf(close), 1);
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div ref={root} className="relative">
      {trigger({ open, toggle: () => setOpen((v) => !v) })}

      {open &&
        host.current &&
        createPortal(
          /* Колонка со своей прокруткой: пункты, которых больше, чем места,
             прокручиваются здесь. Содержимое со своим прокручиваемым куском
             (список у Dropdown) забирает остаток высоты через flex-1
             и прокручивается внутри себя — так поиск над ним не уезжает. */
          <div
            ref={menu}
            role="menu"
            data-popover
            className="fixed z-50 flex flex-col overflow-y-auto rounded-lg border border-border bg-surface p-1 shadow-popover"
          >
            {children(() => setOpen(false))}
          </div>,
          host.current,
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
