/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/no-mole/123pan-goctl/cmd/file"
	"github.com/no-mole/123pan-goctl/cmd/terrors"
	"github.com/no-mole/123pan-goctl/cmd/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "123pan-goctl",
	Short: "A 123pan command line client",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		loggerConfig := zap.NewProductionConfig()
		loggerConfig.Encoding = "console"
		loggerConfig.DisableStacktrace = true
		loggerConfig.DisableCaller = true
		loggerConfig.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
		utils.Logger, err = loggerConfig.Build()
		if err != nil {
			fmt.Println("init logger error:", err.Error())
			return err
		}
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".123pan")

		viper.SetEnvPrefix("123PAN")
		viper.BindEnv("clientId")
		viper.BindEnv("clientSecret")

		viper.AutomaticEnv()

		err = viper.ReadInConfig()

		if err != nil {
			utils.Logger.Error(`config file not found in  $HOME/.123pan.yaml,try to set it:

		client_id: xxxxx
		client_secret: xxxxx
		
		or 

		export 123PAN_CLIENT_ID=xxxxx 123PAN_CLIENT_SECRET=xxxx
		`)
		}

		var ok1, ok2 bool
		utils.ClientId, ok1 = viper.Get("client_id").(string)
		utils.ClientSecret, ok2 = viper.Get("client_secret").(string)
		if !(ok1 && ok2) || (utils.ClientId == "" || utils.ClientSecret == "") {
			utils.Logger.Error("client_id or client_secret not string", zap.Any("client_id", viper.Get("client_id")), zap.Any("client_secret", viper.Get("client_secret")))
			return terrors.New(terrors.CanNotFormatConfigFIle, nil)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		_ = utils.Logger.Sync()
	},
	Version: "v0.0.1",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(file.FileCommand)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.123pan.yaml)")
	//viper.BindPFlag("", rootCmd.PersistentFlags().Lookup("config"))
	//viper.SetDefault("config", "$HOME/.123pan.yaml")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
