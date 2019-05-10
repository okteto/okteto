import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import colors from 'colors.scss';
import classnames from 'classnames';
import autobind from 'autobind-decorator';

import UserMenu from 'components/UserMenu';
import ShareSpaceDialog from 'containers/Dialogs/ShareSpace';
import DeleteSpaceDialog from 'containers/Dialogs/DeleteSpace';
import Icon from 'components/Icon';
import { logout } from 'actions/session';

import 'containers/SpaceHeader.scss';

class SpaceHeader extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showUserMenu: false
    };
  }

  @autobind
  handleShareSpace() {
    this.shareSpaceDialog.getWrappedInstance().open();
  }

  @autobind
  handleDeleteSpace() {
    this.deleteSpaceDialog.getWrappedInstance().open();
  }

  render() {
    const { user, space } = this.props;
    const isPersonalSpace = space.id === user.id;
    const spaceName = user.id === space.id ? `${user.githubID}'s space` : space.name;

    return (
      <div className="SpaceHeader horizontal layout center">
        <div className="SpaceHeaderName">{spaceName}</div>

        <div className="flex-auto" /> 
        
        <div className="SpaceHeaderActions layout horizontal center">
          <div className="ActionButton" onClick={this.handleShareSpace}>
            <Icon icon="share" size="18" />
            Share
          </div>
          <div 
            className={classnames('ActionButton', { disabled: isPersonalSpace })} 
            onClick={this.handleDeleteSpace}
          >
            <Icon icon="cross" size="18" />
            Delete
          </div>
        </div>

        <div className="SpaceHeaderUser">
          <div className="UserAtom layout horizontal center"
            onClick={() => this.setState({ showUserMenu: true })}>
            <div className="Avatar">
              {!user.avatar && 
                <Icon icon="logo" size="52" color={colors.navyDark} />
              }
              {user.avatar && 
                <img src={user.avatar} width="24" />
              }
            </div>
            <div className="Username">{user.githubID}</div>
            <Icon className="ArrowDownIcon" icon="arrowDown" size="24" color="white" />
          </div>
          {this.state.showUserMenu && 
            <UserMenu
              user={user} 
              onLogout={() => {
                this.props.dispatch(logout());
              }}
              onClose={() => this.setState({ showUserMenu: false })}
            />
          }
        </div>
        
        <ShareSpaceDialog 
          ref={ref => this.shareSpaceDialog = ref}
          space={space}
        />
        
        <DeleteSpaceDialog 
          ref={ref => this.deleteSpaceDialog = ref}
          space={space}
        />

      </div>
    );  
  }
}

SpaceHeader.propTypes = {
  dispatch: PropTypes.func,
  user: PropTypes.object.isRequired,
  space: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    user: state.session.user,
    space: state.spaces.current
  };
})(SpaceHeader);