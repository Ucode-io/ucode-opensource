/**
 * Переключатель языка ДАННЫХ: коды в ряд, выбранный подсвечен.
 *
 * Кодами, а не полными именами: языков у проекта бывает четыре, а места
 * рядом с кнопками таблицы — на два слова. Полное имя показывается
 * подсказкой.
 *
 * Ничего не решает сам: и таблица, и карточка переключают один и тот же
 * язык данных, поэтому значение и обработчик приходят снаружи.
 */
export function LanguageTabs({
  languages,
  value,
  onChange,
}: {
  languages: { code: string; nativeName: string }[];
  value: string;
  onChange: (code: string) => void;
}) {
  // Одному языку переключаться некуда.
  if (languages.length < 2) return null;

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {languages.map((language) => (
        <button
          key={language.code}
          type="button"
          onClick={() => onChange(language.code)}
          title={language.nativeName}
          aria-pressed={value === language.code}
          className={`h-6 rounded-md px-1.5 text-xs transition-colors ${
            value === language.code
              ? "bg-accent-subtle text-accent-text"
              : "text-fg-subtle hover:bg-surface-hover hover:text-fg"
          }`}
        >
          {language.code}
        </button>
      ))}
    </div>
  );
}
