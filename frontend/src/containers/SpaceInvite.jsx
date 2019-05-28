import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import * as EmailValidator from 'email-validator';

import Button from 'components/Button';
import Input from 'components/Input';
import Icon from 'components/Icon';
import Avatar from 'components/Avatar';

import './SpaceInvite.scss';

class SpaceInvite extends Component {
  constructor(props) {
    super(props);
    this.state = {
      currentValue: '',
      members: this.props.members,
      membersCache: this.props.members
    };
  }

  reset() {
    this.setState({
      currentValue: '',
      members: this.props.members,
      membersCache: this.props.members
    });
  }

  getMembers() {
    return this.state.members.map(member => member.username || member.email);
  }

  @autobind
  handleAddMember() {
    const { members, currentValue } = this.state;
    const value = currentValue.trim();
    if (value && !members.find(member => member.email === value)) {
      let newMember = this.state.membersCache.find(member => member.email === value);
      if (!newMember) {
        let isEmail = EmailValidator.validate(value);
        newMember = {
          email: isEmail ? value : '',
          username: !isEmail ? value : '',
          new: true
        };
      }
      this.setState({
        members: [newMember, ...members],
        currentValue: ''
      });
    }

    this.inviteInput.focus();
  }

  @autobind
  handleRemoveMember(member) {
    this.setState({
      members: this.state.members.filter(m => {
        return m.username !== member.username || m.email !== member.email;
      })
    });
  }

  focus() {
    this.inviteInput.focus();
  }

  render() {
    return (
      <div className="SpaceInvite layout vertical">
        <div className="SpaceInviteInput layout horizontal center">
          <Input
            className="flex-auto"
            onChange={value => this.setState({ currentValue: value })}
            onKeyPress={event => {
              if (event.key == 'Enter') {
                this.handleAddMember();
              }
            }}
            ref={ref => this.inviteInput = ref}
            placeholder="email address"
            theme="light"
            value={this.state.currentValue}
          />
          <Button onClick={this.handleAddMember} color="grey" solid>Add</Button>
        </div>
        <div className="SpaceInviteMembers layout vertical">
          {this.state.members.map(member => (
            <div 
              className="Member layout horizontal center" 
              key={member.username + member.email}
            >
              { member.username &&
                <>
                  <Avatar className="UserAvatar" username={member.username} size="18" />
                  {member.username}
                </>
              }
              { !member.username && !member.new &&
                <div className="layout horizontal center">
                  <Icon className="UserAvatar" icon="user" size="18" />
                  <div className="layout vertical">
                    {member.email}
                    <div className="MemberInfo">Invitation sent.</div>
                  </div>
                </div>
              }
              { !member.username && member.new &&
                <div className="layout horizontal center">
                  <Icon className="UserAvatar" icon="user" size="18" />
                  {member.email}
                </div>
              }
              <div className="flex-auto"></div>

              {!member.owner &&
                <div 
                  className="DeleteButton"
                  onClick={() => this.handleRemoveMember(member)}>
                  <Icon icon="trash" size="18" />
                </div>
              }
              {member.owner &&
                <div className="OwnerLabel">Owner</div>
              }
            </div>
          ))}
          {this.state.members.length === 0 && 
            <div className="EmptyMembers layout horizontal center">
              Add members to share your development space.
            </div>
          }
        </div>
      </div>
    );
  }
}

SpaceInvite.defaultProps = {
  members: []
};

SpaceInvite.propTypes = {
  members: PropTypes.arrayOf(PropTypes.object)
};

export default SpaceInvite;