import { IconExternalLink } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { mapLink } from "../model/coords";
import { parsePolygon, polygonCenter, polygonPoints } from "../model/polygon";

/**
 * Область на карте: рисуем её форму, а не JSON.
 *
 * В колонке лежит список координат, и текстом это неотличимая от
 * соседней строка `[[41.31,69.24],[41.32,...`. Форма отвечает на
 * единственный вопрос, который к такой ячейке есть, — «какая область
 * тут задана»; точные координаты видны в раскрытой ячейке.
 *
 * Своей карты по-прежнему нет и по той же причине, что у MAP: чужой
 * скрипт, ключ API в настройках каждого поля и запрос к чужому серверу
 * на каждую строку таблицы. Карта открывается ссылкой.
 */
const SMALL = 20;
const LARGE = 128;

export function PolygonCell({ value, wrap }: { value: unknown; wrap?: boolean }) {
  const { t } = useTranslation();

  const points = parsePolygon(value);

  /*
   * Точек нет — два разных случая. Пустой список («[]», «[[]]») это
   * область, которую завели и не нарисовали: прочерк, как у любого
   * незаполненного поля. Всё остальное показываем как есть: колонка
   * POLYGON это varchar, и прятать её содержимое, потому что оно не
   * нашей формы, нельзя.
   */
  if (!points.length) {
    if (isEmptyGeometry(value)) return <span className="text-fg-subtle">—</span>;

    return (
      <span className={`font-mono text-xs ${wrap ? "break-all" : "truncate"}`}>
        {typeof value === "string" ? value : JSON.stringify(value)}
      </span>
    );
  }

  const size = wrap ? LARGE : SMALL;
  const center = polygonCenter(points);

  const shape = (
    <svg
      viewBox={`0 0 ${size} ${size}`}
      style={{ width: size, height: size }}
      className="shrink-0 overflow-visible"
      role="img"
      aria-label={t("cell.polygonPoints", { count: points.length })}
    >
      {/* Цвета из темы: заливка полупрозрачным акцентом, контур им же. */}
      <polygon
        points={polygonPoints(points, size)}
        className="fill-accent/20 stroke-accent"
        strokeWidth={1}
        strokeLinejoin="round"
      />
    </svg>
  );

  if (!wrap) {
    return (
      <span className="flex min-w-0 items-center gap-1.5">
        {shape}
        <span className="truncate text-xs text-fg-muted tabular-nums">{points.length}</span>
      </span>
    );
  }

  return (
    <span className="flex min-w-0 flex-col items-start gap-1">
      {shape}

      <span className="text-xs text-fg-muted">
        {t("cell.polygonPoints", { count: points.length })}
      </span>

      {center && (
        <a
          href={mapLink(center)}
          target="_blank"
          rel="noreferrer noopener"
          // Клик по ссылке не должен заодно раскрывать ячейку.
          onClick={(event) => event.stopPropagation()}
          className="flex items-center gap-1 text-xs text-accent-text hover:underline"
        >
          <Icon as={IconExternalLink} size={14} />
          {t("cell.openMap")}
        </a>
      )}
    </span>
  );
}

/** Разобралось в массив, а точек нет — область просто пустая. */
function isEmptyGeometry(value: unknown): boolean {
  if (typeof value !== "string") return Array.isArray(value);

  try {
    return Array.isArray(JSON.parse(value));
  } catch {
    return false;
  }
}
