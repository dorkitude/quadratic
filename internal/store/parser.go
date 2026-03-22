package store

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"quadratic/internal/browse"
)

type parsedCheckin struct {
	ID        string
	CreatedAt int64
	Date      string
	Shout     string
	VenueName string
	City      string
	State     string
	Country   string
	Source    string
	Category  string
	People    []string
	Photos    []browse.Photo
}

func parseCheckinFile(path string) ([]byte, parsedCheckin, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, parsedCheckin{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, parsedCheckin{}, err
	}
	return body, parseCheckin(raw), nil
}

func parseCheckin(raw map[string]any) parsedCheckin {
	item := parsedCheckin{
		ID:        stringValue(raw["id"]),
		CreatedAt: int64Value(raw["createdAt"]),
		Shout:     stringValue(raw["shout"]),
	}
	if item.CreatedAt > 0 {
		item.Date = time.Unix(item.CreatedAt, 0).Local().Format(time.RFC3339)
	}
	if source, ok := raw["source"].(map[string]any); ok {
		item.Source = stringValue(source["name"])
	}
	if entities, ok := raw["entities"].([]any); ok {
		for _, entity := range entities {
			record, ok := entity.(map[string]any)
			if !ok || stringValue(record["type"]) != "user" {
				continue
			}
			if id := stringValue(record["id"]); id != "" {
				item.People = append(item.People, id)
			}
		}
	}
	if withList, ok := raw["with"].([]any); ok && len(withList) > 0 {
		item.People = item.People[:0]
		for _, personValue := range withList {
			person, ok := personValue.(map[string]any)
			if !ok {
				continue
			}
			if name := stringValue(person["displayName"]); name != "" {
				item.People = append(item.People, name)
				continue
			}
			if id := stringValue(person["id"]); id != "" {
				item.People = append(item.People, id)
			}
		}
	}
	if venue, ok := raw["venue"].(map[string]any); ok {
		item.VenueName = stringValue(venue["name"])
		if location, ok := venue["location"].(map[string]any); ok {
			item.City = stringValue(location["city"])
			item.State = stringValue(location["state"])
			item.Country = stringValue(location["country"])
		}
		if categories, ok := venue["categories"].([]any); ok {
			for _, value := range categories {
				category, ok := value.(map[string]any)
				if !ok {
					continue
				}
				if item.Category == "" || boolValue(category["primary"]) {
					item.Category = stringValue(category["name"])
				}
			}
		}
	}
	if photos, ok := raw["photos"].(map[string]any); ok {
		if items, ok := photos["items"].([]any); ok {
			for _, photoValue := range items {
				photoMap, ok := photoValue.(map[string]any)
				if !ok {
					continue
				}
				prefix := stringValue(photoMap["prefix"])
				suffix := stringValue(photoMap["suffix"])
				if prefix == "" || suffix == "" {
					continue
				}
				width := int(int64Value(photoMap["width"]))
				height := int(int64Value(photoMap["height"]))
				item.Photos = append(item.Photos, browse.Photo{
					ID:       stringValue(photoMap["id"]),
					URL:      prefix + "original" + suffix,
					ThumbURL: prefix + "500x500" + suffix,
					Width:    width,
					Height:   height,
				})
			}
		}
	}
	return item
}

func (p parsedCheckin) SearchableText() string {
	fields := []string{
		p.ID,
		p.Date,
		p.Shout,
		p.VenueName,
		p.City,
		p.State,
		p.Country,
		p.Source,
		p.Category,
	}
	fields = append(fields, p.People...)
	return strings.ToLower(strings.Join(fields, "\n"))
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func boolValue(value any) bool {
	b, _ := value.(bool)
	return b
}
