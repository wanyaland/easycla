// +build !aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runServer(cmd *cobra.Command, args []string) {
	handler := server()

	errs := make(chan error, 2)
	go func() {
		errs <- http.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("PORT")), handler)
	}()
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	fmt.Println("terminated", <-errs)
}
