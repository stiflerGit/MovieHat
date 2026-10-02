package lottery

import "context"

type Drawer interface {
	Draw(ctx context.Context, arg DrawArg) (DrawRet, error)
	GetProbabilities(ctx context.Context, arg GetProbabilitiesArg) (GetProbabilitiesRet, error)
}

type DrawStorage interface {
	StoreDrawResult(ctx context.Context, arg StoreDrawResultArg) (StoreDrawResultRet, error)
}

type GetProbabilitiesArg struct {
	UserIDs []string
}

type GetProbabilitiesRet struct {
	Probabilities []float64
}

type DrawArg struct {
	UserIDs []string
}

type DrawRet struct {
	UserID string
	Index  int
}

type StoreDrawResultArg struct {
	UserIDs  []string
	WinnerID string
}

type StoreDrawResultRet struct {
}
