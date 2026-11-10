package domain

import "errors"

// Ошибки домена. Слой http переводит их в HTTP-коды, репозиторий — из ошибок БД.
var (
	ErrNotFound     = errors.New("не найдено")
	ErrConflict     = errors.New("конфликт")
	ErrInvalidInput = errors.New("некорректные данные")
	ErrUnauthorized = errors.New("требуется авторизация")
	ErrForbidden    = errors.New("недостаточно прав")
)
