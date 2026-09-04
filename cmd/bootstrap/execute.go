package bootstrap

type Executor func()

func Execute(executor Executor) {
	executor()
}
