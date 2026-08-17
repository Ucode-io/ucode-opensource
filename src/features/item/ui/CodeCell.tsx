import { Suspense, lazy } from "react";
import { barcodeFormat } from "../model/cell-kind";

/**
 * QR и штрихкод: значение показывается кодом, а не строкой.
 *
 * Смысл поля в том, чтобы код прочитал сканер — с экрана или с наклейки.
 * Строка «4680123456789» в ячейке не даёт ни того, ни другого: старая
 * админка рисовала картинку, и это единственное, ради чего у типа есть
 * отдельный рендер.
 *
 * Обе библиотеки грузятся отдельными кусками и только когда такая
 * колонка встретилась: у большинства проектов её нет, а в общем бандле
 * они лежали бы у всех.
 */
const Barcode = lazy(() => import("./barcode"));
const QrCode = lazy(() => import("react-qr-code"));

/** Размер кода: в строке таблицы и в раскрытой карточке. */
const SMALL = 24;
const LARGE = 128;

/**
 * Предел стандарта QR — 2953 байта. Длинное значение рисовать нечем,
 * и библиотека на нём бросает исключение, а не рисует пустоту.
 */
const QR_LIMIT = 2900;

export function CodeCell({
  type,
  value,
  qr,
  big,
}: {
  /** Тип поля: от него зависит кодирование штрихкода. */
  type: string;
  value: string;
  qr?: boolean;
  /** Раскрытая ячейка и drawer: код крупный и с подписью. */
  big?: boolean;
}) {
  const size = big ? LARGE : SMALL;

  // Слишком длинное для QR значение — не ошибка данных: в колонку
  // могли записать ссылку с параметрами. Показываем текстом.
  if (qr && value.length > QR_LIMIT) {
    return <span className={big ? "break-words" : "truncate"}>{value}</span>;
  }

  return (
    <span className={`flex min-w-0 gap-1 ${big ? "flex-col items-start" : "items-center"}`}>
      {/* Место под код на время загрузки куска: без него строка дёргается. */}
      <Suspense fallback={<span style={{ width: size, height: size }} />}>
        {qr ? (
          <QrCode
            value={value}
            size={size}
            // Цвета темы: чёрный на чёрном в тёмной теме — пустой квадрат.
            bgColor="transparent"
            fgColor="currentColor"
            style={{ height: size, width: size }}
          />
        ) : (
          <Barcode
            value={value}
            format={barcodeFormat(type)}
            height={big ? 56 : SMALL}
            // Подпись рисует сама библиотека — но только там, где для неё
            // есть место: в строке высотой 36px её нет.
            showText={Boolean(big)}
          />
        )}
      </Suspense>

      {/* У QR подписи своей нет, а значение нужно: его копируют руками. */}
      {big && qr && <span className="font-mono text-xs break-all">{value}</span>}
    </span>
  );
}
