import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

/**
 * Подсказка по наведению.
 *
 * Своя, а не браузерный `title`: тот появляется через секунду с лишним,
 * рисуется системным шрифтом мимо темы, не открывается с клавиатуры
 * и не умеет ничего, кроме одной строки. На доске, в календаре и на
 * таймлайне подсказка — рабочий инструмент: ею подписаны дни оси,
 * колонки карточки и даты события, и выглядеть она должна как часть
 * интерфейса.
 *
 * Обёртка не занимает места (`display: contents`) — разметку вокруг
 * можно не трогать: ни сетка, ни flex не узнают, что элемент завёрнут.
 * Мерить при этом приходится ребёнка: у элемента без бокса своего
 * прямоугольника нет.
 */

/** Через сколько появляется. Меньше — мигает при проходе курсора мимо. */
const DELAY = 300;

/** Зазор между подсказкой и тем, к чему она относится. */
const GAP = 6;

export function Tooltip({
  label,
  children,
  placement = "top",
}: {
  /** Пусто — подсказки нет вовсе: обёртка становится прозрачной. */
  label: ReactNode;
  children: ReactNode;
  /** Сверху по умолчанию; снизу — когда сверху мешает липкая шапка. */
  placement?: "top" | "bottom";
}) {
  const anchor = useRef<HTMLSpanElement>(null);
  const box = useRef<HTMLDivElement>(null);
  const timer = useRef<number>(0);

  /** Прямоугольник того, к чему привязана подсказка. null — скрыта. */
  const [at, setAt] = useState<DOMRect | null>(null);
  const [placed, setPlaced] = useState<{ left: number; top: number } | null>(null);

  const hide = () => {
    window.clearTimeout(timer.current);
    setAt(null);
    setPlaced(null);
  };

  const show = () => {
    if (!label) return;

    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      /* Ребёнок, а не сама обёртка: у элемента с display: contents
         прямоугольник нулевой. */
      const target = anchor.current?.firstElementChild;
      if (target) setAt(target.getBoundingClientRect());
    }, DELAY);
  };

  useEffect(() => () => window.clearTimeout(timer.current), []);

  /*
   * Место считается после отрисовки: ширина зависит от текста. Слоем,
   * а не сдвигом трансформом — так подсказка не размывается на дробных
   * координатах.
   */
  useLayoutEffect(() => {
    const element = box.current;
    if (!at || !element) return;

    const size = element.getBoundingClientRect();
    const above = placement === "top" ? at.top - size.height - GAP : at.bottom + GAP;
    const below = placement === "top" ? at.bottom + GAP : at.top - size.height - GAP;

    setPlaced({
      left: clamp(at.left + at.width / 2 - size.width / 2, GAP, window.innerWidth - size.width - GAP),
      // Не поместилось с той стороны, куда просили, — становимся с другой.
      top: above >= GAP && above + size.height <= window.innerHeight - GAP ? above : below,
    });
  }, [at, placement]);

  /* Уехало из-под курсора — подсказка врёт: прокрутка её закрывает. */
  useEffect(() => {
    if (!at) return;

    window.addEventListener("scroll", hide, true);
    window.addEventListener("resize", hide);

    return () => {
      window.removeEventListener("scroll", hide, true);
      window.removeEventListener("resize", hide);
    };
  }, [at]);

  return (
    <>
      <span
        ref={anchor}
        style={{ display: "contents" }}
        onPointerEnter={show}
        onPointerLeave={hide}
        /* Нажали — значит уже не читают: подсказка поверх перетаскиваемой
           карточки только мешает. */
        onPointerDown={hide}
        /* С клавиатуры — тоже: фокус всплывает, и отдельного обработчика
           на каждую кнопку не нужно. */
        onFocus={show}
        onBlur={hide}
      >
        {children}
      </span>

      {at &&
        createPortal(
          <div
            ref={box}
            role="tooltip"
            className="text-2xs pointer-events-none fixed z-60 max-w-64 rounded-md bg-fg px-2 py-1 text-bg shadow-popover"
            style={{
              left: placed?.left ?? at.left,
              top: placed?.top ?? at.top,
              // До замера уже в DOM, но показывать на черновом месте нельзя.
              opacity: placed ? 1 : 0,
            }}
          >
            {label}
          </div>,
          document.body,
        )}
    </>
  );
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}
