/**
 * Печатные формы записи: шаблон .docx с переменными `{слаг}`, из него
 * бэкенд собирает PDF по конкретной строке.
 *
 * Настраиваются в панели таблицы (`DocTemplates`), печатаются из самой
 * записи (`PrintButton`).
 */
export { DocTemplates } from "./ui/DocTemplates";
export { PrintButton } from "./ui/PrintButton";
export { useDocTemplates, type DocTemplate } from "./api/templates";
