import { useTranslation } from "react-i18next";
import { IconPicker } from "@/features/icons";
import { Dropdown } from "@/shared/ui/dropdown";
import { useFunctions } from "../api/functions";
import type { FieldDraft } from "../model/field-draft";
import { Labeled } from "./FormulaSettings";

/**
 * Функция поля и (у кнопки) её иконка.
 *
 * Полей с функцией два: BUTTON зовёт её кликом, SCAN_BARCODE — когда
 * сканер дочитал код. Иконка есть только у первого: у сканера в ячейке
 * рисунок кода, а не значок.
 *
 * У кнопки значения нет вовсе — колонка в базе пустая всегда, — поэтому
 * здесь нет ни умолчания, ни проверки ввода: настраивать нечего, кроме
 * действия.
 *
 * Компонент монтируется только у этих двух типов: список функций —
 * отдельный запрос, и у остальных полей он уходил бы в никуда на каждое
 * открытие панели.
 */
export function ButtonSettings({
  draft,
  icon = true,
  onChange,
}: {
  draft: FieldDraft;
  /** Показывать выбор иконки. У сканера её нет. */
  icon?: boolean;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  const { t } = useTranslation();
  const { functions, isLoading } = useFunctions();

  return (
    <div className="flex flex-col gap-1.5 px-2 py-1">
      {icon && (
        <Labeled label={t("button.icon")}>
          {/* Тот же пикер, что у пунктов меню: формат имени общий —
              «tabler:bolt», ссылка или файл из нашего CDN. */}
          <IconPicker
            value={draft.icon}
            type={draft.type}
            onChange={(next) => onChange({ icon: next })}
          />
        </Labeled>
      )}

      <Labeled label={t("button.function")}>
        <Dropdown
          size="sm"
          value={draft.functionId}
          placeholder="—"
          items={[
            /*
              Выбранная функция могла быть удалена — тогда её нет в списке,
              и на кнопке осталось бы пустое место. Держим её отдельной
              строкой: настройка видна, и случайной подмены не происходит.
            */
            ...(draft.functionId && !functions.some((item) => item.id === draft.functionId)
              ? [{ value: draft.functionId, label: draft.functionId }]
              : []),
            ...functions.map((item) => ({ value: item.id, label: item.name })),
          ]}
          onChange={(functionId) => onChange({ functionId })}
        />
      </Labeled>

      {/* Без функции кнопка нарисуется, но ничего не сделает: сказать
          об этом надо в настройках, а не молчать до первого клика. */}
      {!isLoading && !functions.length && (
        <p className="text-2xs text-fg-subtle">{t("button.noFunctions")}</p>
      )}
    </div>
  );
}
