import { useLayoutEffect, useRef, useState } from "react";
import JsBarcode from "jsbarcode";

/**
 * Штрихкод полосами.
 *
 * Отдельным модулем с экспортом по умолчанию — чтобы уехать в свой кусок
 * бандла: jsbarcode это несколько десятков килобайт ради колонки, которой
 * у большинства проектов нет вовсе (см. CodeCell).
 */
export default function Barcode({
  value,
  format,
  height,
  showText,
}: {
  value: string;
  /** CODE128 или CODE39 — см. barcodeFormat. */
  format: string;
  /** Высота полос. Ширину задаёт длина значения, а не мы. */
  height: number;
  /** Подпись значением под полосами. В строке таблицы её негде показать. */
  showText: boolean;
}) {
  const svg = useRef<SVGSVGElement>(null);
  /** Значение не кодируется этим форматом — тогда показываем его текстом. */
  const [valid, setValid] = useState(true);

  useLayoutEffect(() => {
    const element = svg.current;
    if (!element) return;

    JsBarcode(element, value, {
      format,
      height,
      width: 1.4,
      margin: 0,
      displayValue: showText,
      fontSize: 11,
      /*
       * Цвета из темы, а не свои: чёрные полосы на тёмном фоне — это
       * пустая ячейка. `currentColor` наследуется от текста ячейки,
       * поэтому код перекрашивается вместе с ней.
       */
      background: "transparent",
      lineColor: "currentColor",
      /*
       * Без своего `valid` библиотека БРОСАЕТ исключение на значении,
       * которое в формат не укладывается (CODE39 не знает строчных букв,
       * EAN считает контрольную цифру). Одна такая строка в данных
       * уронила бы рендер всей таблицы.
       */
      valid: setValid,
    });

    /*
     * jsbarcode проставляет размер в пикселях, и длинный код просто
     * вылезает за колонку шириной 180px. Переводим размер в viewBox:
     * дальше размером управляет CSS, а рисунок ужимается целиком,
     * а не обрезается наполовину — половина штрихкода не сканируется.
     */
    const drawnWidth = element.getAttribute("width");
    const drawnHeight = element.getAttribute("height");
    if (!drawnWidth || !drawnHeight) return;

    element.setAttribute("viewBox", `0 0 ${drawnWidth} ${drawnHeight}`);
    element.removeAttribute("width");
    element.removeAttribute("height");
  }, [value, format, height, showText]);

  return (
    <>
      {/* svg остаётся в разметке и когда значение не кодируется: иначе
          ref пропадает, и следующее — правильное — значение рисовать
          уже некуда. */}
      <svg
        ref={svg}
        role="img"
        aria-label={value}
        style={{ height }}
        className={valid ? "max-w-full" : "hidden"}
      />

      {!valid && <span className="truncate font-mono text-xs">{value}</span>}
    </>
  );
}
