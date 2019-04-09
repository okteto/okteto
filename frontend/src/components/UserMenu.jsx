import React, { Component } from 'react';
import PropTypes from 'prop-types';
import { GoogleLogout } from 'react-google-login';
import autobind from 'autobind-decorator';
import * as clipboard from 'clipboard-polyfill';
import mixpanel from 'mixpanel-browser';

import Button from 'components/Button';
import Icon from 'components/Icon';
import { notify } from 'components/Notification';
import { getToken } from 'common/environment';

import 'components/UserMenu.scss';

class UserMenu extends Component {
  constructor(props) {
    super(props);

    this.state = {
      moreMenuOpen: false
    };
  }

  openMoreMenu() {
    this.setState({ moreMenuOpen: true });
  }

  closeMoreMenu() {
    this.setState({ moreMenuOpen: false });
  }

  @autobind
  handleMoreClick(event) {
    if (!this.state.moreMenuOpen) {
      this.openMoreMenu();
      event.stopPropagation();
    }
  }

  @autobind
  handleOutsideClick() {
    if (this.state.moreMenuOpen) {
      this.closeMoreMenu();
    }
  }

  @autobind
  handleDeleteAccountClick(event) {
    this.closeMoreMenu();
    if (this.props.onDeleteAccount) {
      this.props.onDeleteAccount();
      event.stopPropagation();
    }
  }
  
  render() {
    return (
      <div 
        className="UserMenu layout vertical" 
        onClick={this.handleOutsideClick}>
        <div className="layout horizontal center">
          <img className="avatar" src={this.props.user.pictureURL} />
          <div className="layout vertical">
            <div className="name">{this.props.user.name}</div>
            <div className="email">{this.props.user.email}</div>
          </div>
        </div>
        
        <div className="token layout vertical">
          <label>API Token:</label>
          <div className="layout horizontal center">
            <input className="flex-auto" type="text" value={getToken()} readOnly />
            <Button
              className="clipboard-button"
              icon="copy"
              title="Copy to clipboard"
              onClick={() => {
                clipboard.writeText(getToken());
                notify('Copied to clipboard!');
                mixpanel.track('Copied API Token');
              }}>
            </Button>
          </div>
        </div>
        <div className="actions layout horizontal center">
          <div className="logout-wrapper layout horizontal center-center flex-auto"
            onClick={this.props.onLogout}>
            <GoogleLogout
              className="Button logout-button layout horizontal flex-auto center-center">
            </GoogleLogout>
          </div>
          <div className="more-menu layout horizontal center">
            <Button onClick={this.handleMoreClick}>
              <Icon icon="ellipsis-h" />
            </Button>
            {/* TODO: Fast implementation. We should have generic drop and menu 
                components for this. */}
            {this.state.moreMenuOpen && 
              <div className="more-menu-dropdown">
                <div
                  className="option layout horizontal center"
                  onClick={this.handleDeleteAccountClick}>
                  <Icon icon="trash" color="black" />
                  Delete Account
                </div>
              </div>
            }
          </div>
        </div>
      </div>
    );
  }
}

UserMenu.propTypes = {
  user: PropTypes.object.isRequired,
  onLogout: PropTypes.func.isRequired,
  onDeleteAccount: PropTypes.func.isRequired
};

export default UserMenu;