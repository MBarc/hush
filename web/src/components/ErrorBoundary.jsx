import { Component } from 'react'

// Catches render/lifecycle errors in a page so one bad component shows a
// recoverable message instead of blanking the whole app. Reset it by
// keying it on the route (see App.jsx), so navigating away clears the error.
export class ErrorBoundary extends Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }

  static getDerivedStateFromError(error) {
    return { error }
  }

  componentDidCatch(error, info) {
    console.error('Hush UI error:', error, info)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 p-8 text-center">
          <p className="text-lg font-semibold text-primary">Something went wrong on this page.</p>
          <p className="max-w-md text-sm text-muted">
            Your vault data is safe. Switch to another section, or reload the page.
          </p>
          <button className="btn-primary" onClick={() => window.location.reload()}>
            Reload
          </button>
          {this.state.error?.message && (
            <p className="mono max-w-lg break-all text-xs text-danger/70">
              {String(this.state.error.message)}
            </p>
          )}
        </div>
      )
    }
    return this.props.children
  }
}
