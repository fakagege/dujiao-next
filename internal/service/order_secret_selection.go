package service

import (
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

func normalizeSelectedSecretIDsStrict(ids []uint) ([]uint, error) {
	normalized := normalizeCardSecretIDs(ids)
	if len(normalized) != len(ids) {
		return nil, ErrCardSecretInvalid
	}
	return normalized, nil
}

func buildCreateOrderItemKey(item CreateOrderItem) (string, error) {
	normalizedIDs, err := normalizeSelectedSecretIDsStrict(item.SelectedSecretIDs)
	if err != nil {
		return "", err
	}
	if len(normalizedIDs) == 0 {
		return buildOrderItemKey(item.ProductID, item.SKUID), nil
	}
	parts := make([]string, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return buildOrderItemKey(item.ProductID, item.SKUID) + "|" + strings.Join(parts, ","), nil
}

func (s *OrderService) resolveSelectedSecrets(product *models.Product, sku *models.ProductSKU, item CreateOrderItem) ([]models.CardSecret, error) {
	normalizedIDs, err := normalizeSelectedSecretIDsStrict(item.SelectedSecretIDs)
	if err != nil {
		return nil, err
	}
	if len(normalizedIDs) == 0 {
		return nil, nil
	}
	if product == nil || sku == nil || s.cardSecretRepo == nil {
		return nil, ErrCardSecretInvalid
	}
	if !product.EnableSecretSelection || strings.TrimSpace(product.FulfillmentType) != constants.FulfillmentTypeAuto {
		return nil, ErrCardSecretInvalid
	}
	if item.Quantity != len(normalizedIDs) {
		return nil, ErrInvalidOrderItem
	}
	rows, err := s.cardSecretRepo.ListByIDs(normalizedIDs)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(normalizedIDs) {
		return nil, ErrCardSecretInvalid
	}
	byID := make(map[uint]models.CardSecret, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	selected := make([]models.CardSecret, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		row, ok := byID[id]
		if !ok {
			return nil, ErrCardSecretInvalid
		}
		if row.ProductID != product.ID || row.SKUID != sku.ID || !row.IsSelectable {
			return nil, ErrCardSecretInvalid
		}
		if strings.TrimSpace(row.DisplaySecret) == "" {
			return nil, ErrCardSecretInvalid
		}
		if row.Status != models.CardSecretStatusAvailable {
			return nil, ErrCardSecretInsufficient
		}
		selected = append(selected, row)
	}
	return selected, nil
}

func buildSelectedSecretSnapshot(secrets []models.CardSecret, markup models.Money) models.JSON {
	if len(secrets) == 0 {
		return models.JSON{}
	}
	items := make([]interface{}, 0, len(secrets))
	ids := make([]interface{}, 0, len(secrets))
	for _, secret := range secrets {
		items = append(items, map[string]interface{}{
			"id":             secret.ID,
			"display_secret": secret.DisplaySecret,
		})
		ids = append(ids, secret.ID)
	}
	return models.JSON{
		"count":         len(secrets),
		"markup_amount": markup.String(),
		"secret_ids":    ids,
		"items":         items,
	}
}

func hasSelectedSecretSnapshot(snapshot models.JSON) bool {
	return len(extractSelectedSecretIDs(snapshot)) > 0
}

func extractSelectedSecretIDs(snapshot models.JSON) []uint {
	if len(snapshot) == 0 {
		return nil
	}
	rawIDs, ok := snapshot["secret_ids"]
	if !ok {
		return nil
	}
	list, ok := rawIDs.([]interface{})
	if !ok {
		return nil
	}
	result := make([]uint, 0, len(list))
	for _, raw := range list {
		switch value := raw.(type) {
		case float64:
			if value > 0 {
				result = append(result, uint(value))
			}
		case int:
			if value > 0 {
				result = append(result, uint(value))
			}
		case uint:
			if value > 0 {
				result = append(result, value)
			}
		}
	}
	return result
}
