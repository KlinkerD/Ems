// Package contracts enthält die Typen, die alle Schnittstellenverträge
// gemeinsam nutzen. Die fachlichen Verträge liegen in Unterpaketen wie
// contracts/meter.
//
// Das Modul importiert nur die Standardbibliothek.
package contracts

// Topic ist ein konkretes Broker-Topic, z. B. "meter/umg-1/values".
// Der Broker veröffentlicht und abonniert ausschließlich diesen Typ.
type Topic string
