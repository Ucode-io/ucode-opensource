import { Suspense, lazy, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { IconPrinter } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
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

      {/* Смысл поля со штрихкодом — наклейка на товаре, и напечатать её
          больше негде: печатная форма таблицы подставляет в docx строку
          «4680123456789», а не рисунок. */}
      {big && <PrintCodes type={type} value={value} qr={Boolean(qr)} />}
    </span>
  );
}

/** Сколько наклеек на лист за раз. Больше — это уже типография. */
const MAX_COPIES = 100;

/**
 * Печать наклеек: сколько штук и на принтер.
 *
 * Никакой библиотеки печати: `window.print()` — родная и единственная,
 * а лист собирается тем же рендером кода, что и на экране. Старая
 * админка тянула ради этого `ReactToPrint` и умела печатать только
 * CODABAR (`ElementGenerators/CodabarBarcode/index.jsx:101`).
 *
 * Лист — портал в <body> с классом `print-sheet`: печатается он один,
 * остальной экран прячет правило в app/styles.css. На экране лист
 * невидим всегда — он существует ровно между нажатием и печатью.
 */
function PrintCodes({ type, value, qr }: { type: string; value: string; qr: boolean }) {
  const { t } = useTranslation();
  const [copies, setCopies] = useState(1);
  const [sheet, setSheet] = useState(0);

  /*
   * Печать зовётся ПОСЛЕ того, как копии оказались в DOM, — отсюда
   * состояние вместо прямого вызова. Эффекты потомков React выполняет
   * раньше родительских, поэтому к этому моменту jsbarcode уже нарисовал
   * каждый код: пустой лист на бумагу не уйдёт.
   */
  useEffect(() => {
    if (!sheet) return;

    window.print();
    setSheet(0);
  }, [sheet]);

  return (
    <span className="flex items-center gap-1">
      <input
        type="number"
        min={1}
        max={MAX_COPIES}
        value={copies}
        aria-label={t("cell.printCount")}
        onClick={(event) => event.stopPropagation()}
        onChange={(event) =>
          setCopies(Math.min(MAX_COPIES, Math.max(1, Number(event.target.value) || 1)))
        }
        className="h-6 w-12 rounded-md border border-border-strong bg-surface px-1 text-center text-xs tabular-nums text-fg outline-none focus:border-accent"
      />

      <button
        type="button"
        onClick={(event) => {
          // Клик по кнопке — только печать: ячейка не раскрывается.
          event.stopPropagation();
          setSheet(copies);
        }}
        className="flex h-6 items-center gap-1 rounded-md px-1.5 text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
      >
        <Icon as={IconPrinter} size={14} />
        {t("cell.print")}
      </button>

      {Boolean(sheet) &&
        createPortal(
          <div className="print-sheet flex-wrap content-start gap-4 p-4 text-black">
            {/* Своя граница: кусок с библиотекой к этому моменту уже
                загружен (код показан на экране), но общая граница выше
                — это весь экран, и мигать им ради печати нельзя. */}
            <Suspense fallback={null}>
              {Array.from({ length: sheet }, (_, index) => (
                <span key={index} className="flex flex-col items-center gap-1">
                  {qr ? (
                    /* На бумаге чёрным по белому: currentColor унаследовал бы
                       цвет темы, а светло-серый код сканер не читает. */
                    <QrCode value={value} size={LARGE} bgColor="#fff" fgColor="#000" />
                  ) : (
                    <Barcode value={value} format={barcodeFormat(type)} height={56} showText />
                  )}
                  {qr && <span className="font-mono text-xs break-all">{value}</span>}
                </span>
              ))}
            </Suspense>
          </div>,
          document.body,
        )}
    </span>
  );
}
