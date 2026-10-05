package store

// RefreshFaultySelectSQL is the faulty-set query, exported for the plan test.
var RefreshFaultySelectSQL = faultyPairsSQL("$1::timestamptz", "$2", "$3", "$4")
