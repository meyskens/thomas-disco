module github.com/meyskens/thomas-disco

go 1.16

require (
	github.com/bwmarrin/discordgo v0.23.2
	github.com/davecgh/go-spew v1.1.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/itfactory-tm/thomas-bot v0.0.0-20210909081053-68fca2ab1608
	github.com/jonas747/dca v0.0.0-20201113050843-65838623978b
	github.com/jonas747/ogg v0.0.0-20161220051205-b4f6f4cf3757 // indirect
	github.com/kkdai/youtube/v2 v2.7.4
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.0
	google.golang.org/api v0.56.0
)

// pull select code
replace github.com/bwmarrin/discordgo v0.23.2 => github.com/meyskens/discordgo v0.23.3-0.20210723093830-80a9f1364942
