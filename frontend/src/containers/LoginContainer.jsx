import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import { CSSTransition } from 'react-transition-group';
import GitHubLogin from 'react-github-login';

import Icon from 'components/Icon';
import { notify } from 'components/Notification';
import { loginWithGithub } from 'actions/session';
import environment from 'common/environment';

import 'containers/LoginContainer.scss';
import colors from 'colors.scss';

class LoginContainer extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  onLoginSuccess(response) {
    if (response.code) {
      this.props.dispatch(loginWithGithub(response.code));
    } else {
      notify(`Login Failed: Wrong or missing Github token_id`, 'error');
    }
  }

  @autobind
  onLoginFailure(response) {
    notify(`Login Failed: ${response.error}`, 'error');
  }

  render() {
    return (
      <CSSTransition
        in={true}
        classNames="fade"
        appear={true}
        timeout={5000}>
        <div className="LoginContainer">
          <div className="layout vertical center-center">
            <div className="logo">
              <Icon icon="okteto" size="64" />
            </div>

            <GitHubLogin 
              className="Button LoginButton"
              clientId={environment.githubClientId}
              onSuccess={this.onLoginSuccess}
              onFailure={this.onLoginFailure}>
              <Icon icon="github" size="20" color={colors.white900} />
              Login with Github
            </GitHubLogin>

            <div className="terms">
              By proceeding, you agree to the 
              the <a href="https://okteto.com/legal">Terms of Service</a> and<br /> acknowledge you 
              have read the <a href="https://okteto.com/privacy-policy">Privacy Policy</a>.
            </div>
          </div>
        </div>
      </CSSTransition>
    );
  }
}

LoginContainer.propTypes = {
  session: PropTypes.object.isRequired,
  dispatch: PropTypes.func.isRequired
};

export default ReactRedux.connect(state => {
  return {
    session: state.session
  };
})(LoginContainer);