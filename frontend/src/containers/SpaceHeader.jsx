import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import colors from 'colors.scss';
import classnames from 'classnames';
import autobind from 'autobind-decorator';

import UserMenu from 'components/UserMenu';
import ShareSpaceDialog from 'containers/Dialogs/ShareSpace';
import DeleteSpaceDialog from 'containers/Dialogs/DeleteSpace';
import LeaveSpaceDialog from 'containers/Dialogs/LeaveSpace';
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

  @autobind
  handleLeaveSpace() {
    this.leaveSpaceDialog.getWrappedInstance().open();
  }

  render() {
    const { user, space } = this.props;
    const owner = space.members.find(member => member.owner);
    const isPersonalSpace = user.githubID === space.id;
    const isOwner = owner && owner.id === user.id;

    return (
      <div className="SpaceHeader horizontal layout center">
        <div className="SpaceHeaderInfo layout vertical">
          <div className="SpaceHeaderName">
            {space.id}
            {!isOwner && 
              <span className="Owner">
                @{owner.githubID}
              </span>
            }
          </div>
          
          { isPersonalSpace && 
            <div className="SpaceHeaderSubtitle">
              Personal Namespace
            </div>
          }
        </div>

        <div className="flex-auto" /> 
        
        <div className="SpaceHeaderActions layout horizontal center">
          <div 
            className={classnames('ActionButton', { disabled: !isOwner })} 
            onClick={this.handleShareSpace}>
            <Icon icon="share" size="18" />
            Share
          </div>

          {isOwner &&
            <div 
              className={classnames('ActionButton', { disabled: isPersonalSpace })} 
              onClick={this.handleDeleteSpace}
            >
              <Icon icon="cross" size="18" />
              Delete
            </div>
          }

          {!isOwner &&
            <div 
              className={classnames('ActionButton')} 
              onClick={this.handleLeaveSpace}
            >
              <Icon icon="exit" size="18" />
              Leave
            </div>
          }
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

        <LeaveSpaceDialog 
          ref={ref => this.leaveSpaceDialog = ref}
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