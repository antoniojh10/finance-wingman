package demo

// This file is the demo data, as fixture tables that are easy to tweak for
// screenshots. Everything is fictional. Amounts are minor units (cents) and
// every date is relative to today, resolved in seed.go.

// text is a string in English and Spanish.
type text [2]string

func (t text) in(locale string) string {
	if locale == LocaleES {
		return t[1]
	}
	return t[0]
}

// Account keys used by the fixtures below.
const (
	accChecking = "checking"
	accSavings  = "savings"
	accInvest   = "invest"
	accMXN      = "mxn"
	accJoint    = "joint"
	accCard     = "card"
)

type accountSpec struct {
	key      string
	name     text
	typ      string
	currency string
	shared   bool
	// opening is the balance the day before the history starts.
	opening int64
}

var accounts = []accountSpec{
	{accChecking, text{"Everyday Checking", "Cuenta corriente"}, "checking", "EUR", false, 240_000},
	{accSavings, text{"Emergency Savings", "Ahorro de emergencia"}, "savings", "EUR", false, 800_000},
	{accInvest, text{"Index Fund", "Fondo indexado"}, "investment", "EUR", false, 1_500_000},
	{accMXN, text{"Mexico Checking", "Cuenta en México"}, "checking", "MXN", false, 1_200_000},
	{accJoint, text{"Joint Household", "Cuenta conjunta"}, "checking", "EUR", true, 50_000},
	{accCard, text{"Visa Card", "Tarjeta Visa"}, "credit_card", "EUR", false, 0},
}

// Category keys used by the fixtures below.
const (
	catRent          = "rent"
	catGroceries     = "groceries"
	catTransport     = "transport"
	catRestaurants   = "restaurants"
	catUtilities     = "utilities"
	catLeisure       = "leisure"
	catHealth        = "health"
	catShopping      = "shopping"
	catSubscriptions = "subscriptions"
	catTravel        = "travel"
	catSalary        = "salary"
	catFreelance     = "freelance"
	catInterest      = "interest"
)

type categorySpec struct {
	key   string
	name  text
	kind  string
	color string
	icon  string
}

var categories = []categorySpec{
	{catRent, text{"Rent", "Alquiler"}, "expense", "#6d4aff", "home"},
	{catGroceries, text{"Groceries", "Supermercado"}, "expense", "#12a150", "shopping-cart"},
	{catTransport, text{"Transport", "Transporte"}, "expense", "#2f8cff", "bus"},
	{catRestaurants, text{"Restaurants", "Restaurantes"}, "expense", "#ff6b4a", "utensils"},
	{catUtilities, text{"Utilities", "Suministros"}, "expense", "#ffb020", "zap"},
	{catLeisure, text{"Leisure", "Ocio"}, "expense", "#f05aa6", "ticket"},
	{catHealth, text{"Health", "Salud"}, "expense", "#14b8a6", "heart-pulse"},
	{catShopping, text{"Shopping", "Compras"}, "expense", "#9aa0b8", "shopping-bag"},
	{catSubscriptions, text{"Subscriptions", "Suscripciones"}, "expense", "#6d4aff", "repeat"},
	{catTravel, text{"Travel", "Viajes"}, "expense", "#2f8cff", "plane"},
	{catSalary, text{"Salary", "Nómina"}, "income", "#12a150", "briefcase"},
	{catFreelance, text{"Freelance", "Trabajos extra"}, "income", "#14b8a6", "laptop"},
	{catInterest, text{"Interest", "Intereses"}, "income", "#ffb020", "piggy-bank"},
}

// txSpec is a plain expense or income. It happens on each of the given
// days in each of the given months (0 is the current month, 5 the oldest);
// no months means every month. Descriptions rotate across occurrences and
// amounts vary by up to spread percent. Occurrences after today are
// skipped.
type txSpec struct {
	kind     string
	account  string
	category string // empty: uncategorized
	descs    []text
	amount   int64
	spread   int
	days     []int
	months   []int
}

var transactions = []txSpec{
	// Groceries beyond the receipts: the older months have no receipts.
	{"expense", accChecking, catGroceries, []text{{"Weekly shop", "Compra semanal"}, {"Corner market", "Tienda de barrio"}}, 7_200, 25, []int{4, 11, 18, 25}, []int{3, 4, 5}},
	{"expense", accChecking, catGroceries, []text{{"Corner market", "Tienda de barrio"}}, 1_850, 30, []int{21}, []int{0, 1, 2}},
	{"expense", accJoint, catGroceries, []text{{"Household supplies", "Artículos del hogar"}}, 3_400, 30, []int{9, 23}, nil},

	{"expense", accChecking, catTransport, []text{{"Metro pass", "Abono transporte"}}, 5_400, 0, []int{2}, nil},
	{"expense", accCard, catTransport, []text{{"Ride share", "Taxi compartido"}, {"Train ticket", "Billete de tren"}}, 1_900, 45, []int{7, 19}, nil},

	{"expense", accCard, catRestaurants, []text{{"Trattoria Luna", "Trattoria Luna"}, {"Sushi Hana", "Sushi Hana"}, {"Café Aurora brunch", "Brunch en Café Aurora"}, {"Taco Corner", "Taco Corner"}}, 3_600, 40, []int{3, 10, 17, 24}, nil},
	{"expense", accCard, catRestaurants, []text{{"Birthday dinner", "Cena de cumpleaños"}}, 8_500, 10, []int{15}, []int{4, 1}},

	{"expense", accJoint, catUtilities, []text{{"Electricity", "Electricidad"}}, 6_800, 35, []int{12}, nil},
	{"expense", accJoint, catUtilities, []text{{"Water", "Agua"}}, 3_100, 30, []int{16}, []int{5, 3, 1}},
	{"expense", accJoint, catUtilities, []text{{"Internet", "Internet"}}, 3_990, 0, []int{8}, nil},

	{"expense", accCard, catLeisure, []text{{"Cinema", "Cine"}, {"Concert tickets", "Entradas de concierto"}, {"Bookstore", "Librería"}, {"Board game night", "Noche de juegos de mesa"}}, 2_400, 55, []int{6, 14, 22}, nil},
	{"expense", accCard, catHealth, []text{{"Pharmacy", "Farmacia"}}, 1_650, 40, []int{13}, nil},
	{"expense", accCard, catShopping, []text{{"Clothing store", "Tienda de ropa"}, {"Home decor", "Decoración"}, {"Electronics shop", "Tienda de electrónica"}}, 5_200, 50, []int{20}, nil},
	// The pattern the recurring detector suggests: same text, amount and
	// roughly the same day every month, not tracked as a recurring item.
	{"expense", accChecking, catSubscriptions, []text{{"Cloud Backup", "Cloud Backup"}}, 499, 0, []int{9}, nil},
	{"expense", accCard, catTravel, []text{{"Weekend trip: hotel", "Escapada: hotel"}}, 21_800, 0, []int{12}, []int{2}},
	{"expense", accCard, catTravel, []text{{"Weekend trip: train", "Escapada: tren"}}, 6_400, 0, []int{10}, []int{2}},

	// Uncategorized.
	{"expense", accChecking, "", []text{{"Cash withdrawal", "Retirada de efectivo"}}, 6_000, 0, []int{15}, []int{3, 0}},
	{"expense", accCard, "", []text{{"Unrecognized charge", "Cargo sin identificar"}}, 1_290, 0, []int{23}, []int{1}},

	{"income", accChecking, catFreelance, []text{{"Logo design project", "Proyecto de diseño de logo"}, {"Website consulting", "Consultoría web"}}, 38_000, 35, []int{18}, []int{4, 2, 1}},
	{"income", accSavings, catInterest, []text{{"Savings interest", "Intereses de ahorro"}}, 390, 12, []int{27}, nil},

	// Mexico account (MXN).
	{"income", accMXN, catFreelance, []text{{"Side project payout", "Pago de proyecto"}}, 850_000, 15, []int{5}, nil},
	{"expense", accMXN, catGroceries, []text{{"Mercado local", "Mercado local"}}, 98_000, 30, []int{6, 20}, nil},
	{"expense", accMXN, catRestaurants, []text{{"Tacos al pastor", "Tacos al pastor"}, {"Cafetería Roma", "Cafetería Roma"}}, 38_000, 40, []int{9, 16, 27}, nil},
	{"expense", accMXN, catTransport, []text{{"Metro card top-up", "Recarga de tarjeta de metro"}}, 15_000, 20, []int{3}, nil},
}

// transferSpec moves money between accounts. A zero toAmount means the same
// amount arrives; currencies that differ need it.
type transferSpec struct {
	from, to string
	desc     text
	amount   int64
	toAmount int64
	day      int
	months   []int
}

var transfers = []transferSpec{
	{accChecking, accSavings, text{"Monthly savings", "Ahorro mensual"}, 30_000, 0, 5, nil},
	{accChecking, accInvest, text{"Index fund contribution", "Aportación al fondo indexado"}, 20_000, 0, 6, nil},
	{accChecking, accJoint, text{"Joint account top-up", "Aportación a la cuenta conjunta"}, 45_000, 0, 3, nil},
	{accChecking, accMXN, text{"Send money to Mexico", "Envío a México"}, 20_000, 428_000, 19, []int{4, 2, 0}},
}

// cardPaymentDay is the day of the month the previous month's card spending
// is paid from the checking account.
const cardPaymentDay = 5

var cardPaymentDesc = text{"Credit card payment", "Pago de tarjeta de crédito"}

// budgetSpec is a monthly budget. Budgets apply from the oldest month and
// later months inherit them. The over and near categories are adjusted for
// the current month from what has been spent so far.
type budgetSpec struct {
	category string
	amount   int64
}

var budgets = []budgetSpec{
	{catGroceries, 45_000},
	{catTransport, 12_000},
	{catRestaurants, 22_000},
	{catUtilities, 18_000},
	{catLeisure, 15_000},
	{catShopping, 15_000},
	{catHealth, 8_000},
	{catSubscriptions, 4_000},
}

const (
	overBudgetCategory = catRestaurants
	nearBudgetCategory = catLeisure
)

// recurringSpec is a recurring item. Its past due dates are registered as
// paid, except the one that is left overdue.
type recurringSpec struct {
	kind     string
	account  string
	category string
	name     text
	amount   int64
	// day is the day of the month it is due. Zero: dueIn or overdueBy
	// decide.
	day int
	// dueIn places the next due date that many days from today.
	dueIn int
	// overdueBy places the latest due date that many days before today and
	// leaves it unpaid.
	overdueBy int
}

var recurring = []recurringSpec{
	{kind: "expense", account: accChecking, category: catRent, name: text{"Rent", "Alquiler"}, amount: 85_000, day: 1},
	{kind: "income", account: accChecking, category: catSalary, name: text{"Salary", "Nómina"}, amount: 285_000, day: 28},
	{kind: "expense", account: accCard, category: catSubscriptions, name: text{"StreamBox", "StreamBox"}, amount: 1_299, day: 14},
	{kind: "expense", account: accCard, category: catHealth, name: text{"Gym membership", "Gimnasio"}, amount: 2_990, dueIn: 2},
	{kind: "expense", account: accChecking, category: catUtilities, name: text{"Phone plan", "Plan móvil"}, amount: 1_900, overdueBy: 3},
}

// Store names (fictional chains).
const (
	storeFresh = "FreshMart"
	storeVerde = "Verde Market"
	storeHub   = "PriceHub"
)

// stores lists the stores in the order prices are given in productSpec.
var stores = []string{storeFresh, storeVerde, storeHub}

// productSpec is a product and its single variant. Prices per store (in
// stores order, minor units) are of the pack, or of one compare unit when
// the variant is loose (packSize 0).
type productSpec struct {
	key      string
	name     text
	group    string // default group name, in English
	unit     string // kg, l or unit
	brand    string
	packSize float64
	label    string
	prices   [3]int64
}

var products = []productSpec{
	{"milk", text{"Whole milk", "Leche entera"}, "Dairy & eggs", "l", "Farmhouse", 1, "1 L", [3]int64{109, 119, 89}},
	{"eggs", text{"Eggs", "Huevos"}, "Dairy & eggs", "unit", "Farmhouse", 12, "12 pack", [3]int64{289, 329, 249}},
	{"yogurt", text{"Greek yogurt", "Yogur griego"}, "Dairy & eggs", "unit", "Farmhouse", 4, "4 pack", [3]int64{279, 315, 235}},
	{"bread", text{"Sourdough loaf", "Pan de masa madre"}, "Bakery", "unit", "Generic", 1, "loaf", [3]int64{249, 310, 199}},
	{"bananas", text{"Bananas", "Plátanos"}, "Produce", "kg", "Generic", 0, "", [3]int64{159, 189, 139}},
	{"tomatoes", text{"Tomatoes", "Tomates"}, "Produce", "kg", "Generic", 0, "", [3]int64{299, 349, 259}},
	{"chicken", text{"Chicken breast", "Pechuga de pollo"}, "Meat & fish", "kg", "Generic", 0, "", [3]int64{849, 999, 769}},
	{"pasta", text{"Spaghetti", "Espagueti"}, "Pantry", "kg", "Nonna", 0.5, "500 g", [3]int64{119, 139, 95}},
	{"rice", text{"Rice", "Arroz"}, "Pantry", "kg", "Golden", 1, "1 kg", [3]int64{169, 199, 145}},
	{"oil", text{"Olive oil", "Aceite de oliva"}, "Pantry", "l", "Olivar", 1, "1 L", [3]int64{799, 899, 749}},
	{"coffee", text{"Ground coffee", "Café molido"}, "Pantry", "kg", "Morning", 0.25, "250 g", [3]int64{459, 519, 399}},
	{"detergent", text{"Laundry detergent", "Detergente"}, "Cleaning & household", "l", "Sparkle", 2, "2 L", [3]int64{699, 799, 599}},
	{"paper", text{"Toilet paper", "Papel higiénico"}, "Cleaning & household", "unit", "Generic", 12, "12 rolls", [3]int64{549, 629, 479}},
}

// item is a receipt line: a product and how many packs (or kg when loose).
type item struct {
	product string
	qty     float64
}

// receiptSpec is a grocery receipt from daysAgo days ago, charged to an
// account and linked to an expense. Line amounts come from the product
// prices of the store, with a small variation.
type receiptSpec struct {
	store   string
	daysAgo int
	account string
	items   []item
}

var receipts = []receiptSpec{
	{storeFresh, 2, accChecking, []item{{"milk", 2}, {"eggs", 1}, {"bread", 1}, {"bananas", 1.2}, {"tomatoes", 0.8}, {"chicken", 0.9}, {"coffee", 1}}},
	{storeHub, 8, accChecking, []item{{"pasta", 3}, {"rice", 1}, {"oil", 1}, {"yogurt", 2}, {"milk", 3}, {"paper", 1}}},
	{storeVerde, 15, accCard, []item{{"bananas", 1.5}, {"tomatoes", 1.1}, {"chicken", 1.2}, {"bread", 2}, {"eggs", 1}}},
	{storeFresh, 22, accChecking, []item{{"milk", 2}, {"yogurt", 1}, {"pasta", 2}, {"coffee", 1}, {"detergent", 1}, {"bananas", 1}}},
	{storeHub, 31, accJoint, []item{{"rice", 2}, {"oil", 1}, {"paper", 1}, {"detergent", 1}, {"eggs", 2}, {"milk", 4}}},
	{storeVerde, 39, accChecking, []item{{"tomatoes", 1.4}, {"chicken", 1.0}, {"bread", 1}, {"yogurt", 2}, {"coffee", 1}}},
	{storeFresh, 47, accChecking, []item{{"milk", 2}, {"eggs", 1}, {"bananas", 1.3}, {"pasta", 2}, {"oil", 1}, {"chicken", 0.8}}},
	{storeHub, 58, accChecking, []item{{"paper", 1}, {"detergent", 1}, {"rice", 1}, {"milk", 3}, {"yogurt", 3}, {"bread", 1}}},
	{storeFresh, 67, accCard, []item{{"tomatoes", 0.9}, {"bananas", 1.1}, {"eggs", 1}, {"coffee", 1}, {"pasta", 1}}},
	{storeVerde, 78, accChecking, []item{{"chicken", 1.1}, {"bread", 2}, {"milk", 2}, {"oil", 1}, {"tomatoes", 1.2}, {"yogurt", 2}}},
}

// priceCheckSpec is a price seen in a store without buying, daysAgo days
// ago. The price is the product's price at that store.
type priceCheckSpec struct {
	product string
	store   string
	daysAgo int
	note    text
}

var priceChecks = []priceCheckSpec{
	{"milk", storeVerde, 4, text{}},
	{"milk", storeHub, 4, text{"On promotion this week", "De oferta esta semana"}},
	{"oil", storeVerde, 6, text{}},
	{"coffee", storeHub, 9, text{"Bigger pack was cheaper per kg", "El formato grande salía más barato por kg"}},
	{"chicken", storeHub, 11, text{}},
	{"eggs", storeVerde, 13, text{}},
	{"yogurt", storeHub, 13, text{}},
	{"detergent", storeFresh, 18, text{"Two for one", "2x1"}},
}
