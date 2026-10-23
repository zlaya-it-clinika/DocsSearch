package httpapi

import (
	"net/http"
	"strings"

	"docssearch/core/internal/auth"
	"docssearch/core/internal/livestore"
	"docssearch/core/internal/texts"
)

// Разбор запросов регистрации, LDAP и настроек входа. Общий для заглушки и боевого режима,
// чтобы оба отвечали одинаково.

const maxLdapPort = 65535

type publicAuthConfigResponse struct {
	Onboarding   bool   `json:"onboarding"`
	EnableSignup bool   `json:"enableSignup"`
	EnableLdap   bool   `json:"enableLdap"`
	LdapLabel    string `json:"ldapLabel"`
}

type signupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type ldapLoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type adminDetailsResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type signupSettingsRequest struct {
	EnableSignup     *bool   `json:"enableSignup"`
	DefaultUserRole  *string `json:"defaultUserRole"`
	ShowAdminDetails *bool   `json:"showAdminDetails"`
}

type ldapToggleBody struct {
	EnableLdap *bool `json:"enableLdap"`
}

type ldapToggleResponse struct {
	EnableLdap bool `json:"enableLdap"`
}

type ldapServerRequest struct {
	Label                string `json:"label"`
	Host                 string `json:"host"`
	Port                 *int   `json:"port"`
	AttributeForMail     string `json:"attributeForMail"`
	AttributeForUsername string `json:"attributeForUsername"`
	AppDN                string `json:"appDn"`
	AppPassword          string `json:"appDnPassword"`
	SearchBase           string `json:"searchBase"`
	SearchFilters        string `json:"searchFilters"`
	UseTLS               *bool  `json:"useTls"`
	CertificatePath      string `json:"certificatePath"`
	ValidateCert         *bool  `json:"validateCert"`
	Ciphers              string `json:"ciphers"`
}

// Пароль служебной учётной записи наружу не отдаём, только признак, что он задан
type ldapServerResponse struct {
	Label                string `json:"label"`
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	AttributeForMail     string `json:"attributeForMail"`
	AttributeForUsername string `json:"attributeForUsername"`
	AppDN                string `json:"appDn"`
	AppPassword          string `json:"appDnPassword"`
	AppPasswordSet       bool   `json:"appDnPasswordSet"`
	SearchBase           string `json:"searchBase"`
	SearchFilters        string `json:"searchFilters"`
	UseTLS               bool   `json:"useTls"`
	CertificatePath      string `json:"certificatePath"`
	ValidateCert         bool   `json:"validateCert"`
	Ciphers              string `json:"ciphers"`
}

type createUserRequest struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	AccessLevel *int   `json:"accessLevel"`
	Role        string `json:"role"`
}

type patchUserRequest struct {
	DisplayName *string `json:"displayName"`
	Login       *string `json:"login"`
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	AccessLevel *int    `json:"accessLevel"`
	Role        *string `json:"role"`
}

// Готовая учётная запись для создания: логин, пароль, имя, уровень и признак ожидания
type newAccount struct {
	login       string
	password    string
	displayName string
	accessLevel int
	pending     bool
}

// Вход по паролю: поле login, для удобства принимаем и email. Пустой логин отклонит хранилище
func read_LoginRequest(responseWriter http.ResponseWriter, request *http.Request) (string, string, bool) {
	var body struct {
		Login    string `json:"login"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		return "", "", false
	}
	loginName := strings.TrimSpace(body.Login)
	if loginName == "" {
		loginName = strings.TrimSpace(body.Email)
	}
	return loginName, body.Password, true
}

// Второй результат - код ошибки для ответа 400, пусто - запрос корректный
func read_SignupRequest(responseWriter http.ResponseWriter, request *http.Request) (signupRequest, string) {
	var body signupRequest
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		return signupRequest{}, texts.ErrorInvalidBody
	}
	body.Email = auth.Normalize_Email(body.Email)
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.DisplayName == "" || !auth.Check_PasswordFits(body.Password) {
		return signupRequest{}, texts.ErrorInvalidBody
	}
	if !auth.Check_IsEmail(body.Email) {
		return signupRequest{}, texts.ErrorInvalidEmail
	}
	return body, ""
}

func read_LdapLoginRequest(responseWriter http.ResponseWriter, request *http.Request) (ldapLoginRequest, bool) {
	var body ldapLoginRequest
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		return ldapLoginRequest{}, false
	}
	body.Login = strings.TrimSpace(body.Login)
	return body, true
}

// Частичное изменение: незаданные поля остаются прежними
func apply_SignupSettingsRequest(current auth.SignupSettings, body signupSettingsRequest) (auth.SignupSettings, bool) {
	if body.EnableSignup == nil && body.DefaultUserRole == nil && body.ShowAdminDetails == nil {
		return current, false
	}
	if body.EnableSignup != nil {
		current.EnableSignup = *body.EnableSignup
	}
	if body.DefaultUserRole != nil {
		role := auth.Normalize_Role(*body.DefaultUserRole)
		if !auth.Check_IsKnownRole(role) {
			return current, false
		}
		current.DefaultUserRole = role
	}
	if body.ShowAdminDetails != nil {
		current.ShowAdminDetails = *body.ShowAdminDetails
	}
	return current, true
}

// Обязательные поля как в open-webui: название, хост, атрибуты почты и логина, база поиска
func apply_LdapServerRequest(current auth.LdapSettings, body ldapServerRequest) (auth.LdapSettings, bool) {
	updated := current
	updated.Label = strings.TrimSpace(body.Label)
	updated.Host = strings.TrimSpace(body.Host)
	updated.AttributeForMail = strings.TrimSpace(body.AttributeForMail)
	updated.AttributeForUsername = strings.TrimSpace(body.AttributeForUsername)
	updated.SearchBase = strings.TrimSpace(body.SearchBase)
	if updated.Label == "" || updated.Host == "" || updated.AttributeForMail == "" || updated.AttributeForUsername == "" || updated.SearchBase == "" {
		return current, false
	}
	updated.Port = 0
	if body.Port != nil {
		if *body.Port < 0 || *body.Port > maxLdapPort {
			return current, false
		}
		updated.Port = *body.Port
	}
	updated.AppDN = strings.TrimSpace(body.AppDN)
	switch {
	case updated.AppDN == "":
		// Без служебной учётной записи пароль не нужен
		updated.AppPassword = ""
	case body.AppPassword != "":
		updated.AppPassword = body.AppPassword
	}
	updated.SearchFilters = strings.TrimSpace(body.SearchFilters)
	if body.UseTLS != nil {
		updated.UseTLS = *body.UseTLS
	}
	updated.CertificatePath = strings.TrimSpace(body.CertificatePath)
	if body.ValidateCert != nil {
		updated.ValidateCert = *body.ValidateCert
	}
	updated.Ciphers = strings.TrimSpace(body.Ciphers)
	if !auth.Check_CiphersKnown(updated.Ciphers) {
		return current, false
	}
	return updated, true
}

func build_LdapServerResponse(settings auth.LdapSettings) ldapServerResponse {
	return ldapServerResponse{
		Label:                settings.Label,
		Host:                 settings.Host,
		Port:                 settings.Port,
		AttributeForMail:     settings.AttributeForMail,
		AttributeForUsername: settings.AttributeForUsername,
		AppDN:                settings.AppDN,
		AppPassword:          "",
		AppPasswordSet:       settings.AppPassword != "",
		SearchBase:           settings.SearchBase,
		SearchFilters:        settings.SearchFilters,
		UseTLS:               settings.UseTLS,
		CertificatePath:      settings.CertificatePath,
		ValidateCert:         settings.ValidateCert,
		Ciphers:              settings.Ciphers,
	}
}

// Почту приводим к нижнему регистру, старый логин без @ оставляем как есть
func normalize_AccountLogin(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.Contains(trimmed, "@") {
		return auth.Normalize_Email(trimmed)
	}
	return trimmed
}

// Роль важнее уровня: role задаёт и уровень, и признак ожидания. Без роли - уровень из запроса
func read_CreateUserRequest(responseWriter http.ResponseWriter, request *http.Request) (newAccount, bool) {
	var body createUserRequest
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		return newAccount{}, false
	}
	login := body.Login
	if strings.TrimSpace(login) == "" {
		login = body.Email
	}
	created := newAccount{
		login:       normalize_AccountLogin(login),
		password:    body.Password,
		displayName: strings.TrimSpace(body.DisplayName),
		accessLevel: texts.SeedDocumentAccessOpen,
	}
	if created.login == "" || created.displayName == "" || !auth.Check_PasswordFits(created.password) {
		return newAccount{}, false
	}
	if body.AccessLevel != nil {
		created.accessLevel = *body.AccessLevel
	}
	if role := auth.Normalize_Role(body.Role); role != "" {
		if !auth.Check_IsKnownRole(role) {
			return newAccount{}, false
		}
		created.accessLevel, created.pending = auth.Access_ForRole(role)
	}
	return created, true
}

func read_PatchUserRequest(responseWriter http.ResponseWriter, request *http.Request) (livestore.AccountPatch, bool) {
	var body patchUserRequest
	if err := read_JSONBody(responseWriter, request, &body); err != nil {
		return livestore.AccountPatch{}, false
	}
	patch := livestore.AccountPatch{AccessLevel: body.AccessLevel}
	if body.DisplayName != nil {
		displayName := strings.TrimSpace(*body.DisplayName)
		if displayName == "" {
			return livestore.AccountPatch{}, false
		}
		patch.DisplayName = &displayName
	}
	loginSource := body.Login
	if loginSource == nil {
		loginSource = body.Email
	}
	if loginSource != nil {
		login := normalize_AccountLogin(*loginSource)
		if login == "" {
			return livestore.AccountPatch{}, false
		}
		patch.Login = &login
	}
	if body.Password != nil {
		if !auth.Check_PasswordFits(*body.Password) {
			return livestore.AccountPatch{}, false
		}
		patch.Password = body.Password
	}
	if body.Role != nil {
		role := auth.Normalize_Role(*body.Role)
		if !auth.Check_IsKnownRole(role) {
			return livestore.AccountPatch{}, false
		}
		accessLevel, pending := auth.Access_ForRole(role)
		if patch.AccessLevel == nil {
			patch.AccessLevel = &accessLevel
		}
		patch.Pending = &pending
	}
	if patch.DisplayName == nil && patch.Login == nil && patch.Password == nil && patch.AccessLevel == nil && patch.Pending == nil {
		return livestore.AccountPatch{}, false
	}
	return patch, true
}
