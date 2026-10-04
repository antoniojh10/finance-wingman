package finance

import "context"

type Currency struct {
	Code       string `json:"code" example:"MXN"`
	Name       string `json:"name" example:"Mexican Peso"`
	Symbol     string `json:"symbol" example:"$"`
	MinorUnits int    `json:"minor_units" example:"2" doc:"Number of decimal places; amounts are expressed in these minor units"`
}

func (s *Service) ListCurrencies(ctx context.Context) ([]Currency, error) {
	rows, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Currency, len(rows))
	for i, r := range rows {
		out[i] = Currency{Code: r.Code, Name: r.Name, Symbol: r.Symbol, MinorUnits: int(r.MinorUnits)}
	}
	return out, nil
}
