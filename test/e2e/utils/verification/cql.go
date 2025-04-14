// Copyright (C) 2025 ScyllaDB

package verification

import (
	"context"

	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
)

func InsertAndVerifyCQLData(ctx context.Context, hosts []string, options ...utils.DataInserterOption) *utils.DataInserter {
	framework.By("Inserting data")
	di, err := utils.NewDataInserter(hosts, options...)
	o.Expect(err).NotTo(o.HaveOccurred())

	InsertAndVerifyCQLDataUsingDataInserter(ctx, di)
	return di
}

func InsertAndVerifyCQLDataByDC(ctx context.Context, hosts map[string][]string) *utils.DataInserter {
	di, err := utils.NewMultiDCDataInserter(hosts)
	o.Expect(err).NotTo(o.HaveOccurred())

	InsertAndVerifyCQLDataUsingDataInserter(ctx, di)
	return di
}

func InsertAndVerifyCQLDataUsingDataInserter(ctx context.Context, di *utils.DataInserter) *utils.DataInserter {
	framework.By("Inserting data")
	err := di.Insert()
	o.Expect(err).NotTo(o.HaveOccurred())

	VerifyCQLData(ctx, di)

	return di
}

func VerifyCQLData(ctx context.Context, di *utils.DataInserter) {
	err := di.AwaitSchemaAgreement(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Verifying the data")
	data, err := di.Read()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(data).To(o.Equal(di.GetExpected()))
}
