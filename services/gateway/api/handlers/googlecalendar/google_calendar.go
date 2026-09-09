package googlecalendar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	DefaultCalendarID = "primary"
	SyncDirection     = "ucode_to_google"

	EventIDField      = "google_calendar_event_id"
	CalendarIDField   = "google_calendar_id"
	SyncStatusField   = "google_calendar_sync_status"
	LastSyncedAtField = "google_calendar_last_synced_at"
	LastErrorField    = "google_calendar_last_error"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type Client interface {
	CreateEvent(ctx context.Context, credentials *pb.GoogleCalendarCredentials, event *calendar.Event) (*calendar.Event, error)
	UpdateEvent(ctx context.Context, credentials *pb.GoogleCalendarCredentials, eventID string, event *calendar.Event) (*calendar.Event, error)
	DeleteEvent(ctx context.Context, credentials *pb.GoogleCalendarCredentials, eventID string) error
}

type APIClient struct {
	Config Config
}

type SyncRequest struct {
	CompanyServices services.CompanyServiceI
	Services        services.ServiceManagerI
	Resource        *pb.ServiceResourceModel
	ProjectID       string
	EnvironmentID   string
	TableSlug       string
	Data            map[string]any
	Client          Client
	Config          Config
}

func NewOAuthConfig(config Config) (*oauth2.Config, error) {
	if strings.TrimSpace(config.ClientID) == "" {
		return nil, errors.New("GOOGLE_CALENDAR_CLIENT_ID is empty")
	}
	if strings.TrimSpace(config.ClientSecret) == "" {
		return nil, errors.New("GOOGLE_CALENDAR_CLIENT_SECRET is empty")
	}
	if strings.TrimSpace(config.RedirectURI) == "" {
		return nil, errors.New("GOOGLE_CALENDAR_REDIRECT_URI is empty")
	}
	return &oauth2.Config{
		ClientID:     strings.TrimSpace(config.ClientID),
		ClientSecret: strings.TrimSpace(config.ClientSecret),
		RedirectURL:  strings.TrimSpace(config.RedirectURI),
		Scopes:       []string{calendar.CalendarEventsScope},
		Endpoint:     google.Endpoint,
	}, nil
}

func CredentialsFromRefreshToken(refreshToken string) (*pb.GoogleCalendarCredentials, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("refresh token is empty")
	}
	return &pb.GoogleCalendarCredentials{
		AuthType:      "oauth",
		CalendarId:    DefaultCalendarID,
		RefreshToken:  strings.TrimSpace(refreshToken),
		SyncDirection: SyncDirection,
	}, nil
}

func ValidateMapping(mapping *pb.GoogleCalendarMapping) error {
	if mapping == nil {
		return errors.New("google calendar mapping is required")
	}
	if strings.TrimSpace(mapping.GetTableSlug()) == "" {
		return errors.New("table_slug is required")
	}
	if strings.TrimSpace(mapping.GetTitleField()) == "" {
		return errors.New("title_field is required")
	}
	if strings.TrimSpace(mapping.GetStartField()) == "" {
		return errors.New("start_field is required")
	}
	if strings.TrimSpace(mapping.GetEndField()) == "" {
		return errors.New("end_field is required")
	}
	return nil
}

func GetConfiguredResource(ctx context.Context, companyServices services.CompanyServiceI, projectID, environmentID string) (*pb.ProjectResource, *pb.GoogleCalendarCredentials, bool, error) {
	list, err := companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId:     projectID,
		EnvironmentId: environmentID,
		Type:          pb.ResourceType_GOOGLE_CALENDAR,
	})
	if err != nil {
		if IsUnsupportedResourceTypeError(err) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	if len(list.GetResources()) == 0 {
		return nil, nil, false, nil
	}
	if len(list.GetResources()) > 1 {
		return nil, nil, false, errors.New("multiple google calendar resources configured for project environment")
	}
	resource := list.GetResources()[0]
	if resource.GetSettings() == nil || resource.GetSettings().GetGoogleCalendar() == nil {
		return resource, nil, false, nil
	}
	credentials := resource.GetSettings().GetGoogleCalendar()
	if strings.TrimSpace(credentials.GetCalendarId()) == "" {
		credentials.CalendarId = DefaultCalendarID
	}
	return resource, credentials, true, nil
}

func EnsureHiddenFields(ctx context.Context, services services.ServiceManagerI, resourceEnvID string, mapping *pb.GoogleCalendarMapping) error {
	if err := ValidateMapping(mapping); err != nil {
		return err
	}
	table, err := findTableBySlug(ctx, services, resourceEnvID, mapping.GetTableSlug())
	if err != nil {
		return err
	}
	return EnsureHiddenFieldsForTable(ctx, services, resourceEnvID, table, mapping)
}

func EnsureHiddenFieldsForTable(ctx context.Context, services services.ServiceManagerI, resourceEnvID string, table *nb.Table, mapping *pb.GoogleCalendarMapping) error {
	if table == nil {
		return errors.New("table is required")
	}
	resourceEnvID = strings.TrimSpace(resourceEnvID)
	if resourceEnvID == "" {
		return errors.New("resource environment id is required")
	}
	tableID := strings.TrimSpace(table.GetId())
	if tableID == "" {
		return errors.New("table id is required")
	}
	if strings.TrimSpace(mapping.GetTableSlug()) == "" {
		mapping.TableSlug = table.GetSlug()
	}
	if err := ValidateMapping(mapping); err != nil {
		return err
	}
	fields, err := services.GoObjectBuilderService().Field().GetAll(ctx, &nb.GetAllFieldsRequest{
		Limit:     1000,
		TableSlug: mapping.GetTableSlug(),
		ProjectId: resourceEnvID,
	})
	if err != nil {
		return fmt.Errorf("get fields for table %q: %w", mapping.GetTableSlug(), err)
	}

	existing := make(map[string]bool, len(fields.GetFields()))
	fieldTypes := make(map[string]string, len(fields.GetFields()))
	for _, field := range fields.GetFields() {
		existing[field.GetSlug()] = true
		fieldTypes[field.GetSlug()] = field.GetType()
	}
	if err := validateMappingFields(mapping, fieldTypes); err != nil {
		return err
	}

	for _, field := range []struct {
		slug  string
		label string
		typ   string
	}{
		{EventIDField, "Google Calendar Event ID", "SINGLE_LINE"},
		{CalendarIDField, "Google Calendar ID", "SINGLE_LINE"},
		{SyncStatusField, "Google Calendar Sync Status", "SINGLE_LINE"},
		{LastSyncedAtField, "Google Calendar Last Synced At", "DATE_TIME"},
		{LastErrorField, "Google Calendar Last Error", "MULTI_LINE"},
	} {
		if existing[field.slug] {
			continue
		}
		attributes, _ := helper.ConvertMapToStruct(map[string]any{
			"label_en": field.label,
			"hidden":   true,
			"system":   true,
		})
		if _, err := services.GoObjectBuilderService().Field().Create(ctx, &nb.CreateFieldRequest{
			Id:         uuid.NewString(),
			TableId:    tableID,
			ProjectId:  resourceEnvID,
			Slug:       field.slug,
			Label:      field.label,
			Type:       field.typ,
			IsVisible:  false,
			Attributes: attributes,
		}); err != nil {
			return fmt.Errorf("create hidden field %q for table_id %q project_id %q: %w", field.slug, tableID, resourceEnvID, err)
		}
	}

	return nil
}

func ResolveTable(ctx context.Context, services services.ServiceManagerI, resourceEnvID, tableID, tableSlug string) (*nb.Table, error) {
	tableID = strings.TrimSpace(tableID)
	tableSlug = strings.TrimSpace(tableSlug)
	if strings.TrimSpace(resourceEnvID) == "" {
		return nil, errors.New("resource environment id is required")
	}
	if tableID != "" {
		table, err := services.GoObjectBuilderService().Table().GetByID(ctx, &nb.TablePrimaryKey{
			Id:        tableID,
			ProjectId: resourceEnvID,
		})
		if err != nil {
			return nil, fmt.Errorf("get table by id %q project_id %q: %w", tableID, resourceEnvID, err)
		}
		if table == nil {
			return nil, fmt.Errorf("table %q not found", tableID)
		}
		if strings.TrimSpace(table.GetId()) == "" {
			table.Id = tableID
		}
		if strings.TrimSpace(table.GetSlug()) == "" {
			table.Slug = tableSlug
		}
		if strings.TrimSpace(table.GetSlug()) == "" {
			return nil, fmt.Errorf("table %q has empty slug; send table_slug with the mapping request", tableID)
		}
		return table, nil
	}
	if tableSlug == "" {
		return nil, errors.New("table_id or table_slug is required")
	}
	return findTableBySlug(ctx, services, resourceEnvID, tableSlug)
}

func validateMappingFields(mapping *pb.GoogleCalendarMapping, fieldTypes map[string]string) error {
	check := func(kind, slug string, required, dateCompatible bool) error {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			if required {
				return fmt.Errorf("%s is required", kind)
			}
			return nil
		}
		fieldType, ok := fieldTypes[slug]
		if !ok {
			return fmt.Errorf("%s %q not found in table %q", kind, slug, mapping.GetTableSlug())
		}
		if dateCompatible && !isDateCompatibleFieldType(fieldType) {
			return fmt.Errorf("%s %q must be DATE or DATE_TIME compatible, got %q", kind, slug, fieldType)
		}
		return nil
	}

	if err := check("title_field", mapping.GetTitleField(), true, false); err != nil {
		return err
	}
	if err := check("start_field", mapping.GetStartField(), true, true); err != nil {
		return err
	}
	if err := check("end_field", mapping.GetEndField(), true, true); err != nil {
		return err
	}
	if err := check("description_field", mapping.GetDescriptionField(), false, false); err != nil {
		return err
	}
	if err := check("location_field", mapping.GetLocationField(), false, false); err != nil {
		return err
	}
	if err := check("attendees_field", mapping.GetAttendeesField(), false, false); err != nil {
		return err
	}
	return check("status_field", mapping.GetStatusField(), false, false)
}

func isDateCompatibleFieldType(fieldType string) bool {
	switch strings.ToUpper(strings.TrimSpace(fieldType)) {
	case "DATE", "DATE_TIME", "DATE_TIME_WITHOUT_TIME_ZONE", "DATE_TIME_WITH_TIME_ZONE", "TIMESTAMP", "TIMESTAMPTZ":
		return true
	default:
		return false
	}
}

func SyncCreate(ctx context.Context, req SyncRequest) error {
	return syncUpsert(ctx, req, false)
}

func SyncUpdate(ctx context.Context, req SyncRequest) error {
	return syncUpsert(ctx, req, true)
}

func SyncDelete(ctx context.Context, req SyncRequest) error {
	if req.Resource == nil || req.Resource.GetResourceType() != pb.ResourceType_POSTGRESQL {
		return nil
	}
	_, credentials, configured, err := GetConfiguredResource(ctx, req.CompanyServices, req.ProjectID, req.EnvironmentID)
	if err != nil || !configured {
		return err
	}
	mapping := credentials.GetMapping()
	if mapping == nil || mapping.GetTableSlug() != req.TableSlug {
		return nil
	}

	eventID := strings.TrimSpace(fmt.Sprint(req.Data[EventIDField]))
	if eventID == "" || eventID == "<nil>" {
		return nil
	}

	client := req.Client
	if client == nil {
		client = APIClient{Config: req.Config}
	}
	return client.DeleteEvent(ctx, credentials, eventID)
}

func syncUpsert(ctx context.Context, req SyncRequest, update bool) error {
	if req.Resource == nil || req.Resource.GetResourceType() != pb.ResourceType_POSTGRESQL {
		return nil
	}
	_, credentials, configured, err := GetConfiguredResource(ctx, req.CompanyServices, req.ProjectID, req.EnvironmentID)
	if err != nil || !configured {
		return err
	}
	mapping := credentials.GetMapping()
	if mapping == nil || mapping.GetTableSlug() != req.TableSlug {
		return nil
	}
	if err := ValidateMapping(mapping); err != nil {
		return err
	}

	client := req.Client
	if client == nil {
		client = APIClient{Config: req.Config}
	}
	event, err := eventFromData(mapping, req.Data)
	if err != nil {
		_ = saveSyncState(ctx, req, "", "error", googleCalendarErrorMessage(err))
		return err
	}

	eventID := strings.TrimSpace(fmt.Sprint(req.Data[EventIDField]))
	if !update || eventID == "" || eventID == "<nil>" {
		created, err := client.CreateEvent(ctx, credentials, event)
		if err != nil {
			_ = saveSyncState(ctx, req, "", "error", googleCalendarErrorMessage(err))
			return err
		}
		return saveSyncState(ctx, req, created.Id, "synced", "")
	}

	updated, err := client.UpdateEvent(ctx, credentials, eventID, event)
	if err != nil {
		_ = saveSyncState(ctx, req, eventID, "error", googleCalendarErrorMessage(err))
		return err
	}
	return saveSyncState(ctx, req, updated.Id, "synced", "")
}

func eventFromData(mapping *pb.GoogleCalendarMapping, data map[string]any) (*calendar.Event, error) {
	title := strings.TrimSpace(fmt.Sprint(data[mapping.GetTitleField()]))
	if title == "" || title == "<nil>" {
		return nil, errors.New("mapped title field is empty")
	}
	start, err := eventDateTime(data[mapping.GetStartField()])
	if err != nil {
		return nil, fmt.Errorf("mapped start field is invalid: %w", err)
	}
	end, err := eventDateTime(data[mapping.GetEndField()])
	if err != nil {
		return nil, fmt.Errorf("mapped end field is invalid: %w", err)
	}

	event := &calendar.Event{
		Summary: title,
		Start:   start,
		End:     end,
	}
	if mapping.GetDescriptionField() != "" {
		event.Description = strings.TrimSpace(fmt.Sprint(data[mapping.GetDescriptionField()]))
	}
	if mapping.GetLocationField() != "" {
		event.Location = strings.TrimSpace(fmt.Sprint(data[mapping.GetLocationField()]))
	}
	if mapping.GetAttendeesField() != "" {
		event.Attendees = attendeesFromValue(data[mapping.GetAttendeesField()])
	}
	if mapping.GetStatusField() != "" {
		if status := googleEventStatus(data[mapping.GetStatusField()]); status != "" {
			event.Status = status
		}
	}
	return event, nil
}

func googleEventStatus(value any) string {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	switch status {
	case "", "<nil>":
		return ""
	case "confirmed", "confirm", "scheduled", "planned", "active", "approved":
		return "confirmed"
	case "tentative", "pending", "draft":
		return "tentative"
	case "cancelled", "canceled", "cancel":
		return "cancelled"
	default:
		return ""
	}
}

func eventDateTime(value any) (*calendar.EventDateTime, error) {
	if typed, ok := value.(time.Time); ok {
		return &calendar.EventDateTime{DateTime: typed.Format(time.RFC3339)}, nil
	}

	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" || raw == "<nil>" {
		return nil, errors.New("date value is empty")
	}
	if len(raw) == len("2006-01-02") {
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return nil, err
		}
		return &calendar.EventDateTime{Date: raw}, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &calendar.EventDateTime{DateTime: t.Format(time.RFC3339)}, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return &calendar.EventDateTime{DateTime: t.Format(time.RFC3339)}, nil
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05.999999999",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &calendar.EventDateTime{DateTime: t.Format(time.RFC3339)}, nil
		}
	}
	return &calendar.EventDateTime{DateTime: raw}, nil
}

func googleCalendarErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if gErr, ok := err.(*googleapi.Error); ok {
		body := strings.TrimSpace(gErr.Body)
		if body != "" {
			return fmt.Sprintf("%s: %s", gErr.Error(), body)
		}
	}
	return err.Error()
}

func attendeesFromValue(value any) []*calendar.EventAttendee {
	switch typed := value.(type) {
	case []any:
		attendees := make([]*calendar.EventAttendee, 0, len(typed))
		for _, item := range typed {
			email := strings.TrimSpace(fmt.Sprint(item))
			if email != "" {
				attendees = append(attendees, &calendar.EventAttendee{Email: email})
			}
		}
		return attendees
	case []string:
		attendees := make([]*calendar.EventAttendee, 0, len(typed))
		for _, email := range typed {
			email = strings.TrimSpace(email)
			if email != "" {
				attendees = append(attendees, &calendar.EventAttendee{Email: email})
			}
		}
		return attendees
	default:
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" || raw == "<nil>" {
			return nil
		}
		parts := strings.Split(raw, ",")
		attendees := make([]*calendar.EventAttendee, 0, len(parts))
		for _, part := range parts {
			email := strings.TrimSpace(part)
			if email != "" {
				attendees = append(attendees, &calendar.EventAttendee{Email: email})
			}
		}
		return attendees
	}
}

func saveSyncState(ctx context.Context, req SyncRequest, eventID, status, lastError string) error {
	guid := strings.TrimSpace(fmt.Sprint(req.Data["guid"]))
	if guid == "" || guid == "<nil>" {
		guid = strings.TrimSpace(fmt.Sprint(req.Data["id"]))
	}
	if guid == "" || guid == "<nil>" {
		return errors.New("cannot save google calendar sync state without row guid")
	}

	data := map[string]any{
		"guid":            guid,
		CalendarIDField:   DefaultCalendarID,
		SyncStatusField:   status,
		LastSyncedAtField: time.Now().Format(time.RFC3339),
		LastErrorField:    lastError,
	}
	if eventID != "" {
		data[EventIDField] = eventID
	}
	structData, err := helper.ConvertMapToStruct(data)
	if err != nil {
		return err
	}
	_, err = req.Services.GoObjectBuilderService().Items().Update(ctx, &nb.CommonMessage{
		TableSlug: req.TableSlug,
		Data:      structData,
		ProjectId: req.Resource.GetResourceEnvironmentId(),
	})
	return err
}

func findTableBySlug(ctx context.Context, services services.ServiceManagerI, resourceEnvID, tableSlug string) (*nb.Table, error) {
	resourceEnvID = strings.TrimSpace(resourceEnvID)
	tableSlug = strings.TrimSpace(tableSlug)
	if resourceEnvID == "" {
		return nil, errors.New("resource environment id is required")
	}
	if tableSlug == "" {
		return nil, errors.New("table_slug is required")
	}
	tables, err := services.GoObjectBuilderService().Table().GetAll(ctx, &nb.GetAllTablesRequest{
		ProjectId: resourceEnvID,
		Limit:     1000,
	})
	if err != nil {
		return nil, fmt.Errorf("get tables for project_id %q: %w", resourceEnvID, err)
	}
	for _, table := range tables.GetTables() {
		if table.GetSlug() == tableSlug {
			return table, nil
		}
	}
	return nil, fmt.Errorf("table %q not found", tableSlug)
}

func (c APIClient) CreateEvent(ctx context.Context, credentials *pb.GoogleCalendarCredentials, event *calendar.Event) (*calendar.Event, error) {
	service, err := c.service(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return service.Events.Insert(calendarID(credentials), event).Do()
}

func (c APIClient) UpdateEvent(ctx context.Context, credentials *pb.GoogleCalendarCredentials, eventID string, event *calendar.Event) (*calendar.Event, error) {
	service, err := c.service(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return service.Events.Update(calendarID(credentials), eventID, event).Do()
}

func (c APIClient) DeleteEvent(ctx context.Context, credentials *pb.GoogleCalendarCredentials, eventID string) error {
	service, err := c.service(ctx, credentials)
	if err != nil {
		return err
	}
	err = service.Events.Delete(calendarID(credentials), eventID).Do()
	if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
		return nil
	}
	return err
}

func (c APIClient) service(ctx context.Context, credentials *pb.GoogleCalendarCredentials) (*calendar.Service, error) {
	config := Config{
		ClientID:     strings.TrimSpace(credentials.GetClientId()),
		ClientSecret: strings.TrimSpace(credentials.GetClientSecret()),
		RedirectURI:  c.Config.RedirectURI,
	}
	if config.ClientID == "" {
		config.ClientID = c.Config.ClientID
	}
	if config.ClientSecret == "" {
		config.ClientSecret = c.Config.ClientSecret
	}
	oauthConfig, err := NewOAuthConfig(config)
	if err != nil {
		return nil, err
	}
	token := &oauth2.Token{RefreshToken: strings.TrimSpace(credentials.GetRefreshToken())}
	client := oauthConfig.Client(ctx, token)
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

func calendarID(credentials *pb.GoogleCalendarCredentials) string {
	id := strings.TrimSpace(credentials.GetCalendarId())
	if id == "" {
		return DefaultCalendarID
	}
	return id
}

func IsUnsupportedResourceTypeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid input value for enum resource_type") &&
		(strings.Contains(msg, `"16"`) || strings.Contains(msg, "google_calendar"))
}

func StructToMap(data *structpb.Struct) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	return data.AsMap()
}
