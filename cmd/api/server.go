package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	//Create a shutdownError channel. We will use this to receive any errors returned
	//by the graceful Shutdown() function.
	shutdownError := make(chan error)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		app.logger.PrintInfo("caught signal", map[string]string{
			"signal": s.String(),
		})
		//Create a context with a 30-second timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		app.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})

		app.wg.Wait()

		shutdownError <- nil
	}()

	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})

	//calling shutdown() on our server will cause listenandserve() to immediately
	//return a http.errserverclosed error. So if we see this error, it is actually a
	//good thing and an indication that the graceful shutdown has started. So we check
	//specially for this, only returning the error if it is not http.errserverclosed.
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	//Otherwise, we wait to receive the return value from Shutdown() on the
	//shutdownError channel. If return value is an error,we know that there was a
	//problem with the graceful shutdown and we return the error
	err = <-shutdownError
	if err != nil {
		return err
	}

	//At this point we know that the graceful shutdown completed successfully and we
	//log a "shopped server" message.
	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil

}
