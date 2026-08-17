import type { KeyboardEvent, PointerEvent, RefObject } from "react";

/**
 * Ручка изменения ширины: у сайдбара — на правом краю, у drawer'а — на
 * левом. Один компонент, потому что разница между ними — знак.
 *
 * Во время перетаскивания ширина пишется прямо в стиль элемента, а
 * наружу отдаётся один раз, на отпускании: ширина персистится в
 * localStorage, и запись на каждое движение мыши — это запись 60 раз
 * в секунду.
 */
export function ResizeHandle({
  edge,
  target,
  value,
  min,
  max,
  label,
  onCommit,
  onDrag,
}: {
  /** Какой край тянем. */
  edge: "left" | "right";
  target: RefObject<HTMLElement | null>;
  value: number;
  min: number;
  max: number;
  label: string;
  onCommit: (width: number) => void;
  /** Вызывается на каждое движение — для реакций вроде «сверни сайдбар». */
  onDrag?: (width: number) => void;
}) {
  const clamp = (width: number) => Math.round(Math.min(max, Math.max(min, width)));

  const startDrag = (event: PointerEvent<HTMLDivElement>) => {
    const el = target.current;
    if (!el || event.button !== 0) return;
    event.preventDefault();

    const box = el.getBoundingClientRect();
    const widthAt = (clientX: number) =>
      clamp(edge === "right" ? clientX - box.left : box.right - clientX);

    const onMove = (move: globalThis.PointerEvent) => {
      const width = widthAt(move.clientX);
      el.style.width = `${width}px`;
      onDrag?.(width);
    };
    const onUp = (up: globalThis.PointerEvent) => {
      document.removeEventListener("pointermove", onMove);
      document.body.style.cursor = "";
      onCommit(widthAt(up.clientX));
    };

    document.body.style.cursor = "col-resize";
    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp, { once: true });
  };

  // Стрелками — тоже: тянуть мышью полосу шириной 6px может не каждый.
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const towards = event.key === "ArrowLeft" ? -1 : event.key === "ArrowRight" ? 1 : 0;
    if (!towards) return;
    event.preventDefault();
    onCommit(clamp(value + towards * 16 * (edge === "right" ? 1 : -1)));
  };

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={value}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onPointerDown={startDrag}
      onKeyDown={onKeyDown}
      className={`group/resize absolute top-0 z-10 flex h-full w-1.5 cursor-col-resize items-center justify-center ${
        edge === "right" ? "right-0" : "left-0"
      }`}
    >
      {/* Видимая только при наведении «пилюля»: полоса во всю высоту
          читается как граница блока и спорит с разделителями таблицы. */}
      <span className="h-8 w-1 rounded-full bg-border-strong opacity-0 transition-opacity group-hover/resize:opacity-100 group-focus-visible/resize:opacity-100" />
    </div>
  );
}
