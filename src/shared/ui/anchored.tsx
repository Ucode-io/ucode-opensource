import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

/**
 * Слой, привязанный к прямоугольнику на экране.
 *
 * Портал в <body>, а не absolute внутри родителя: редактор ячейки живёт
 * внутри области с overflow-auto, и любой всплывающий блок там обрезается
 * её краем — вместо меню появляется полоса прокрутки. Popover из
 * shared/ui для меню под кнопкой годится, для ячейки в прокручиваемой
 * таблице — нет.
 *
 * Закрывается по клику мимо, по прокрутке и по изменению размера окна.
 * Пересчитывать координаты на каждый кадр прокрутки незачем: ячейка
 * под редактором уезжает вместе с ней, и висящий на пустом месте
 * редактор врёт сильнее, чем закрывшийся.
 *
 * Escape отделён от остального закрытия: для редактора это «отменить»,
 * а клик мимо — «сохранить». Кто различает эти два намерения — вызывающий.
 */
const GAP = 8;

export function Anchored({
  anchor,
  onClose,
  onCancel,
  children,
}: {
  anchor: DOMRect;
  /** Клик мимо, прокрутка, resize. Для редактора — «применить». */
  onClose: () => void;
  /** Escape. По умолчанию то же самое. */
  onCancel?: () => void;
  children: ReactNode;
}) {
  const box = useRef<HTMLDivElement>(null);
  const [placed, setPlaced] = useState<{ left: number; top: number } | null>(null);

  /*
   * Позиция считается после отрисовки: высота зависит от содержимого,
   * а место под ячейкой — от того, где она оказалась. useLayoutEffect,
   * чтобы сдвиг успел до кадра: иначе редактор виден дважды — сначала
   * за краем экрана, потом на месте.
   */
  useLayoutEffect(() => {
    const element = box.current;
    if (!element) return;

    const { width, height } = element.getBoundingClientRect();

    setPlaced({
      left: clamp(anchor.left, GAP, window.innerWidth - width - GAP),
      top: clamp(anchor.top, GAP, window.innerHeight - height - GAP),
    });
  }, [anchor]);

  useEffect(() => {
    const cancel = onCancel ?? onClose;

    const onPointerDown = (event: PointerEvent) => {
      if (!box.current?.contains(event.target as Node)) onClose();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") cancel();
    };
    /*
     * Своя прокрутка не в счёт: внутри лежат списки — часы, минуты,
     * варианты выбора, строки связанной таблицы. Закрываться от колеса
     * мыши над собственным списком — значит не давать им прокрутиться.
     */
    const onScroll = (event: Event) => {
      if (!box.current?.contains(event.target as Node)) onClose();
    };

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    // capture: прокрутка внутри таблицы до window не всплывает.
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("resize", onClose);

    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("resize", onClose);
    };
  }, [onClose, onCancel]);

  return createPortal(
    <div
      ref={box}
      className="fixed z-50"
      style={{
        left: placed?.left ?? anchor.left,
        top: placed?.top ?? anchor.top,
        // Редактор не уже ячейки: значение не должно переноситься иначе,
        // чем оно перенесётся после сохранения.
        minWidth: anchor.width,
        /*
         * До замера блок уже в DOM (иначе нечего мерить), но показывать
         * его на предварительном месте нельзя. Прячется прозрачностью,
         * а не visibility: скрытый через visibility элемент не принимает
         * фокус, и autoFocus внутри редактора молча не срабатывал —
         * ячейка открывалась, а первое нажатие клавиши уходило странице.
         */
        opacity: placed ? 1 : 0,
      }}
    >
      {children}
    </div>,
    document.body,
  );
}

function clamp(value: number, min: number, max: number): number {
  // max может оказаться меньше min на узком экране — тогда важнее min.
  return Math.max(min, Math.min(value, max));
}
