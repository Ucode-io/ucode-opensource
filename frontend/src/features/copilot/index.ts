/**
 * Помощник u-code: панель справа, в которой ИИ по просьбе словами
 * заводит таблицы, поля, связи, разделы меню и записи.
 *
 * Панель живёт в оболочке приложения (`routes/_authed`), кнопка — в шапке
 * страницы. Состояние «открыта» общее и лежит в `shared/lib/ui-store`:
 * иначе кнопке и панели пришлось бы знать друг о друге.
 */
export { CopilotPanel, CopilotButton } from "./ui/CopilotPanel";
export { CopilotPage } from "./ui/CopilotPage";
